package provisioner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/pkg/hetzner"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"go.uber.org/zap"
)

const (
	// Servers are created in Germany; the cheapest in-stock x86 type wins.
	serverCountry = "DE"
	// P-Node supports Debian/Ubuntu on amd64; the newest Debian image is used.
	imagePrefix = "debian-"
	nodeSshUser = "root"
	nodeSshPort = 22
	// nodeDir is where the P-Node installer puts the first instance on a fresh server.
	nodeDir = "/root/p-node-1"

	bootTimeout = 5 * time.Minute
	sshTimeout  = 5 * time.Minute
	infoTimeout = 2 * time.Minute
	pollEvery   = 5 * time.Second
)

// installScript installs P-Node with its one-line installer. Fresh cloud
// images run cloud-init and unattended upgrades on first boot, which hold the
// apt lock for a while; the script waits for them instead of failing.
const installScript = `set -e
export DEBIAN_FRONTEND=noninteractive
cloud-init status --wait >/dev/null 2>&1 || true
for i in $(seq 1 120); do
  if fuser /var/lib/dpkg/lock-frontend /var/lib/apt/lists/lock >/dev/null 2>&1; then sleep 5; else break; fi
done
echo 'DPkg::Lock::Timeout "600";' > /etc/apt/apt.conf.d/99p-manager-lock-timeout
cd /root
curl -fsSL https://raw.githubusercontent.com/miladrahimi/p-node/master/scripts/install.sh | bash
`

// nodeInfo is what P-Manager needs from the installed P-Node.
type nodeInfo struct {
	HttpPort  int
	HttpToken string
}

// provision runs the whole flow. It returns the created server (also on
// failure, so the caller can delete it) and the first error.
func (p *Provisioner) provision(ctx context.Context, job *Job, client *hetzner.Client, managerUrl string) (*hetzner.Server, error) {
	nodeId := p.newNodeId()

	p.setStep(job, "Listing the SSH keys")
	keyIds, err := sshKeyIds(ctx, client)
	if err != nil {
		return nil, err
	}

	p.setStep(job, "Choosing the server type")
	serverType, location, err := cheapestServerType(ctx, client)
	if err != nil {
		return nil, err
	}
	image, err := latestImage(ctx, client)
	if err != nil {
		return nil, err
	}

	p.setStep(job, fmt.Sprintf("Creating the server (%s in %s)", serverType, location))
	server, err := client.CreateServer(ctx, hetzner.CreateServerRequest{
		Name:             "p-node-" + nodeId,
		ServerType:       serverType,
		Image:            image,
		Location:         location,
		SshKeys:          keyIds,
		Labels:           map[string]string{"managed-by": "p-manager", "p-manager-node": nodeId},
		PublicNet:        hetzner.PublicNet{EnableIpv4: true, EnableIpv6: false},
		StartAfterCreate: true,
	})
	if err != nil {
		return nil, err
	}

	p.setStep(job, "Waiting for the server to boot")
	if server, err = waitRunning(ctx, client, server.Id); err != nil {
		return server, err
	}
	conn := ssh.NewConnectionConfig(server.Ipv4(), nodeSshUser, nodeSshPort)
	p.ssh.ForgetHost(ctx, server.Ipv4())

	p.setStep(job, "Waiting for SSH")
	if err = p.waitSsh(ctx, conn); err != nil {
		return server, err
	}

	p.setStep(job, "Installing P-Node (this takes a few minutes)")
	if _, err = p.ssh.Run(ctx, conn, installScript); err != nil {
		return server, err
	}

	p.setStep(job, "Reading the P-Node info")
	info, err := p.readNodeInfo(ctx, conn)
	if err != nil {
		return server, err
	}

	p.setStep(job, "Adding the node")
	node, err := p.addNode(nodeId, server, info, serverDetails(server))
	if err != nil {
		return server, err
	}

	// Pulling is configured best-effort: the node is already usable through
	// push and SSH, and the pull command stays available in the node details.
	if managerUrl != "" {
		p.setStep(job, "Configuring pulling")
		command := fmt.Sprintf("make -C %s set-manager URL=%q TOKEN=%q",
			nodeDir, strings.TrimRight(managerUrl, "/")+"/api/node/"+node.Id, node.PullToken)
		if _, err = p.ssh.Run(ctx, conn, command); err != nil {
			p.l.Error("provisioner: cannot configure pulling", zap.String("node", node.Id), zap.Error(err))
		}
	}

	go p.coordinator.CheckSshStatus(node.Id)
	go p.coordinator.UpdateConfigs()

	return server, nil
}

// serverDetails summarizes a Hetzner server for the node details view.
func serverDetails(server *hetzner.Server) map[string]string {
	t := server.ServerType
	details := map[string]string{
		"Provider": "Hetzner Cloud",
		"Server":   fmt.Sprintf("%s (#%d)", server.Name, server.Id),
		"IP":       server.Ipv4(),
		"Type":     t.Name,
		"Specs":    fmt.Sprintf("%d vCPU, %g GB RAM, %d GB disk", t.Cores, t.Memory, t.Disk),
		"Location": fmt.Sprintf("%s (%s, %s)", server.Location.Name, server.Location.City, server.Location.Country),
		"Created":  server.Created,
	}
	if server.Image != nil {
		details["Image"] = server.Image.Name
	}
	if price := t.MonthlyPrice(server.Location.Name); price > 0 {
		details["Price"] = fmt.Sprintf("€%.2f/month gross (€%s/hour net)", price, t.HourlyPriceNet(server.Location.Name))
	}
	return details
}

// sshKeyIds returns the ids of all SSH keys in the Hetzner project. They are
// all injected into the new server; the manager host's key must be among them.
func sshKeyIds(ctx context.Context, client *hetzner.Client) ([]int64, error) {
	keys, err := client.ListSshKeys(ctx)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New("the Hetzner project has no SSH keys; add this host's key to the project first")
	}
	ids := make([]int64, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, k.Id)
	}
	return ids, nil
}

// cheapestServerType picks the cheapest non-deprecated x86 server type that is
// in stock at a location in the configured country, and that location.
func cheapestServerType(ctx context.Context, client *hetzner.Client) (string, string, error) {
	locations, err := client.ListLocations(ctx)
	if err != nil {
		return "", "", err
	}
	wanted := map[string]bool{}
	for _, l := range locations {
		if l.Country == serverCountry {
			wanted[l.Name] = true
		}
	}

	types, err := client.ListServerTypes(ctx)
	if err != nil {
		return "", "", err
	}

	var best *hetzner.ServerType
	var bestPrice float64
	var bestLocation string
	for i := range types {
		t := &types[i]
		if t.Architecture != "x86" || t.Deprecated {
			continue
		}
		for _, l := range t.Locations {
			if !l.Available || !wanted[l.Name] {
				continue
			}
			price := t.MonthlyPrice(l.Name)
			if price <= 0 {
				continue
			}
			better := best == nil || price < bestPrice ||
				(price == bestPrice && (t.Memory > best.Memory || (t.Memory == best.Memory && t.Cores > best.Cores)))
			if better {
				best, bestPrice, bestLocation = t, price, l.Name
			}
		}
	}
	if best == nil {
		return "", "", errors.Newf("no x86 server type is in stock in %s right now", serverCountry)
	}
	return best.Name, bestLocation, nil
}

// latestImage picks the newest available Debian system image.
func latestImage(ctx context.Context, client *hetzner.Client) (string, error) {
	images, err := client.ListSystemImages(ctx)
	if err != nil {
		return "", err
	}

	var best *hetzner.Image
	var bestVersion float64
	for i := range images {
		img := &images[i]
		if !strings.HasPrefix(img.Name, imagePrefix) || img.Deprecated != nil {
			continue
		}
		version, err := strconv.ParseFloat(img.OsVersion, 64)
		if err != nil {
			continue
		}
		if best == nil || version > bestVersion {
			best, bestVersion = img, version
		}
	}
	if best == nil {
		return "", errors.Newf("no %s* image is available", imagePrefix)
	}
	return best.Name, nil
}

// waitRunning polls the server until it is running with a public IPv4.
func waitRunning(ctx context.Context, client *hetzner.Client, id int64) (*hetzner.Server, error) {
	deadline := time.Now().Add(bootTimeout)
	for {
		server, err := client.GetServer(ctx, id)
		if err != nil {
			return nil, err
		}
		if server.Status == "running" && server.Ipv4() != "" {
			return server, nil
		}
		if time.Now().After(deadline) {
			return server, errors.Newf("server is still %q after %s", server.Status, bootTimeout)
		}
		if err = sleep(ctx, pollEvery); err != nil {
			return server, err
		}
	}
}

// waitSsh polls until the manager can log in to the server as root.
func (p *Provisioner) waitSsh(ctx context.Context, conn *ssh.ConnectionConfig) error {
	deadline := time.Now().Add(sshTimeout)
	for {
		attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		_, err := p.ssh.Run(attemptCtx, conn, "true")
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.Wrapf(err, "cannot log in to %s as %s within %s", conn.Host, conn.User, sshTimeout)
		}
		if err = sleep(ctx, pollEvery); err != nil {
			return err
		}
	}
}

// readNodeInfo reads the HTTP port and token from the installed P-Node's
// database, which the service writes shortly after its first start.
func (p *Provisioner) readNodeInfo(ctx context.Context, conn *ssh.ConnectionConfig) (nodeInfo, error) {
	deadline := time.Now().Add(infoTimeout)
	for {
		output, err := p.ssh.Run(ctx, conn, "cat "+nodeDir+"/storage/database/data.json")
		if err == nil {
			var db struct {
				Settings struct {
					HttpPort  int    `json:"http_port"`
					HttpToken string `json:"http_token"`
				} `json:"settings"`
			}
			if jsonErr := json.Unmarshal([]byte(output), &db); jsonErr != nil {
				err = errors.Wrap(jsonErr, "cannot parse the P-Node database")
			} else if db.Settings.HttpPort < 1 || db.Settings.HttpToken == "" {
				err = errors.New("the P-Node database has no HTTP port/token yet")
			} else {
				return nodeInfo{HttpPort: db.Settings.HttpPort, HttpToken: db.Settings.HttpToken}, nil
			}
		}
		if time.Now().After(deadline) {
			return nodeInfo{}, err
		}
		if err = sleep(ctx, pollEvery); err != nil {
			return nodeInfo{}, err
		}
	}
}

// sleep waits for d or until the context is done.
func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return errors.WithStack(ctx.Err())
	case <-time.After(d):
		return nil
	}
}
