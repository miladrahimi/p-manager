package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/http/client"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
	"github.com/xtls/xray-core/app/stats/command"
	"go.uber.org/zap"
)

// Coordinator is the main app component which coordinates the synchronization of the local and remote configs.
type Coordinator struct {
	l        *logger.Logger
	c        *config.Config
	db       *database.Database[data.Data]
	hc       *client.Client
	xray     *xray.Xray
	composer *composer.Composer
	state    *State
	ssh      *ssh.Manager
}

// New creates a new coordinator.
func New(
	config *config.Config,
	hc *client.Client,
	logger *logger.Logger,
	db *database.Database[data.Data],
	xray *xray.Xray,
	writer *composer.Composer,
	sshManager *ssh.Manager,
) *Coordinator {
	return &Coordinator{
		l:        logger,
		hc:       hc,
		c:        config,
		db:       db,
		xray:     xray,
		composer: writer,
		state:    newState(),
		ssh:      sshManager,
	}
}

// Run starts the coordinator and its workers.
func (c *Coordinator) Run(ctx context.Context) error {
	c.l.Info("coordinator: running...")

	if err := c.initialize(); err != nil {
		return errors.WithStack(err)
	}

	c.UpdateConfigs()

	go newWorker("pushConfigToStaleNodes", time.Second*10, c.l, func() {
		c.pushConfigToStaleNodes()
	}).Start(ctx)

	go newWorker("updateNodePullStatuses", time.Minute, c.l, func() {
		if err := c.updateNodePullStatuses(); err != nil {
			c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
		}
	}).Start(ctx)

	go newWorker("loadLocalStats", time.Minute, c.l, func() {
		if err := c.loadLocalStats(); err != nil {
			c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
		}
	}).Start(ctx)

	go newWorker("pullStatsFromNodes", time.Minute, c.l, func() {
		c.pullStatsFromNodes()
	}).Start(ctx)

	go newWorker("Backup", time.Hour, c.l, func() {
		if err := c.db.Backup(); err != nil {
			c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
		}
	}).Start(ctx)

	go newWorker("resetUsageForUsers", time.Hour, c.l, func() {
		if err := c.resetUsageForUsers(); err != nil {
			c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
		}
	}).Start(ctx)

	return nil
}

// initialize initializes the coordinator.
func (c *Coordinator) initialize() (err error) {
	d := c.db.Data()

	if d.XraySettings.VrrvPrivateKey == "" || d.XraySettings.VrrvPublicKey == "" {
		d.XraySettings.VrrvPrivateKey, d.XraySettings.VrrvPublicKey, err = c.xray.GenerateX25519()
		if err != nil {
			return errors.WithStack(err)
		}
	}

	if d.XraySettings.NodeSni == "" {
		d.XraySettings.NodeSni = config.DefaultNodeSni
	}
	if d.XraySettings.ManagerSni == "" {
		d.XraySettings.ManagerSni = config.DefaultManagerSni
	}

	return errors.WithStack(c.db.Save())
}

// UpdateConfigs updates the local and node configs.
func (c *Coordinator) UpdateConfigs() {
	c.l.Info("coordinator: updating configs...")
	c.syncSshProxies()
	if err := c.updateLocalConfig(); err != nil {
		c.l.Fatal("coordinator:", zap.Error(errors.WithStack(err)))
	}
	c.pushConfigToNodes()
}

// updateLocalConfig updates the local xray config.
func (c *Coordinator) updateLocalConfig() error {
	c.l.Info("coordinator: updating local configs...")

	localConfig, err := c.composer.LocalConfig(c.state.SshLocalPorts())
	if err != nil {
		return err
	}

	if err = c.xray.Reconfigure(localConfig); err != nil {
		return errors.WithStack(err)
	}

	c.state.xrayUpdatedAt = time.Now()

	return nil
}

// pushConfigToNodes pushes the config to all nodes.
func (c *Coordinator) pushConfigToNodes() {
	c.l.Info("coordinator: pushing config to all nodes...")
	for _, s := range c.db.Data().Nodes {
		go c.pushConfigToNode(s)
	}
}

// pushConfigToStaleNodes pushes the config to stale nodes.
func (c *Coordinator) pushConfigToStaleNodes() {
	c.l.Info("coordinator: pushing config to stale nodes...")
	for _, n := range c.db.Data().Nodes {
		isStale := n.PushStatus == data.NodeStatusUnavailable || n.PushStatus == data.NodeStatusProcessing
		if isStale {
			go c.pushConfigToNode(n)
		}
	}
}

// pushConfigToNode pushes the config to a single node.
func (c *Coordinator) pushConfigToNode(n *data.Node) {
	url := fmt.Sprintf("%s://%s:%d/xray/config", "http", n.Host, n.HttpPort)
	proxy := c.db.Data().MainSettings.SingetServer
	nc := c.composer.NodeConfig(n, c.state.XrayUpdatedAt())

	proxied := false
	success := false

	_, err := c.hc.Do(http.MethodPost, url, n.HttpToken, nc)
	if err == nil {
		success = true
	} else if proxy != "" {
		proxied = true
		_, err = c.hc.DoThrough(proxy, http.MethodPost, url, n.HttpToken, nc)
		if err == nil {
			success = true
		}
	}

	if success {
		n.PushedAt = time.Now().UnixMilli()
		if proxied {
			n.PushStatus = data.NodeStatusDirty
		} else {
			n.PushStatus = data.NodeStatusAvailable
		}

		c.l.Debug(
			"coordinator: config pushed to a node successfully",
			zap.String("url", url),
			zap.String("proxy", proxy),
			zap.Bool("proxied", proxied),
		)
	} else {
		n.PushStatus = data.NodeStatusUnavailable
		c.l.Error(
			"coordinator: cannot push config to a node",
			zap.String("url", url),
			zap.String("proxy", proxy),
			zap.Bool("proxied", proxied),
			zap.Error(err),
		)
	}

	if err = c.db.Save(); err != nil {
		c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
	}
}

// pullStatsFromNodes pulls the stats of all nodes.
func (c *Coordinator) pullStatsFromNodes() {
	if c.db.Data().XraySettings.VrrvRemotePort == 0 {
		return
	}

	c.l.Info("coordinator: pulling stats from all nodes...")
	for _, s := range c.db.Data().Nodes {
		go c.pullStatsFromNode(s)
	}
}

// pullStatsFromNode pulls the stats of a single node.
func (c *Coordinator) pullStatsFromNode(node *data.Node) {
	url := fmt.Sprintf("%s://%s:%d/xray/stats", "http", node.Host, node.HttpPort)

	c.l.Info("coordinator: pulling stats from a node...", zap.String("url", url))

	response, err := c.hc.Do(http.MethodGet, url, node.HttpToken, nil)
	if err != nil {
		c.l.Error("cannot fetch node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
		return
	}

	var queryStats []*command.Stat
	if err = json.Unmarshal(response, &queryStats); err != nil {
		c.l.Error("cannot read remote node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
		return
	}

	users := map[string]int64{}
	var nodeUsageBytes int64

	for _, qs := range queryStats {
		parts := strings.Split(qs.GetName(), ">>>")
		if len(parts) < 2 {
			continue
		}
		if parts[0] == "user" {
			users[parts[1]] += qs.GetValue()
		} else if parts[0] == "inbound" && parts[1] == "remote" {
			nodeUsageBytes += qs.GetValue()
		}
	}

	shouldSync := false
	db := c.db.Data()
	for _, u := range db.Users {
		if bytes, found := users[u.VlessId]; found {
			u.UsageBytes = util.SafeSumI64(u.UsageBytes, bytes)
			u.Usage = util.Bytes2GB(u.UsageBytes)
			if u.Quota > 0 && u.Usage > u.Quota {
				u.Enabled = false
				shouldSync = true
				c.l.Debug("coordinator: user disabled", zap.String("id", u.Id))
			}
		}
	}
	if shouldSync {
		go c.UpdateConfigs()
	}

	node.UsageBytes = util.SafeSumI64(node.UsageBytes, nodeUsageBytes)
	node.Usage = util.Bytes2GB(node.UsageBytes)

	db.Stats.TotalUsageBytes = util.SafeSumI64(db.Stats.TotalUsageBytes, nodeUsageBytes)
	db.Stats.TotalUsage = util.Bytes2GB(db.Stats.TotalUsageBytes)

	if err = c.db.Save(); err != nil {
		c.l.Error("cannot save remote node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
	}
}

// loadLocalStats loads the local XraySettings instance stats.
func (c *Coordinator) loadLocalStats() error {
	c.l.Info("coordinator: loading local stats...")

	queryStats, err := c.xray.QueryStats()
	if err != nil {
		return errors.WithStack(err)
	}

	nodes := map[string]int64{}
	users := map[string]int64{}

	for _, qs := range queryStats {
		parts := strings.Split(qs.GetName(), ">>>")
		if len(parts) < 2 {
			continue
		}
		if parts[0] == "user" {
			users[parts[1]] += qs.GetValue()
		} else if parts[0] == "inbound" && strings.HasPrefix(parts[1], "internal-") { //TODO
			nodes[parts[1][8:]] += qs.GetValue()
		} else if parts[0] == "outbound" && strings.HasPrefix(parts[1], "relay-") {
			nodes[parts[1][6:]] += qs.GetValue()
		} else if parts[0] == "inbound" && slices.Contains([]string{"reverse", "relay", "direct"}, parts[1]) {
			c.db.Data().Stats.TotalUsageBytes = util.SafeSumI64(c.db.Data().Stats.TotalUsageBytes, qs.GetValue())
		}
	}

	db := c.db.Data()
	for _, n := range db.Nodes {
		if bytes, found := nodes[n.Id]; found {
			n.UsageBytes = util.SafeSumI64(n.UsageBytes, bytes)
		}
		n.Usage = util.Bytes2GB(n.UsageBytes)
	}

	db.Stats.TotalUsage = util.Bytes2GB(db.Stats.TotalUsageBytes)

	shouldSync := false
	for _, u := range db.Users {
		if bytes, found := users[u.VlessId]; found {
			u.UsageBytes = util.SafeSumI64(u.UsageBytes, bytes)
			u.Usage = util.Bytes2GB(u.UsageBytes)
			if u.Quota > 0 && u.Usage > u.Quota {
				u.Enabled = false
				shouldSync = true
				c.l.Debug("coordinator: user disabled", zap.String("id", u.Id))
			}
		}
	}
	if shouldSync {
		go c.UpdateConfigs()
	}

	err = c.db.Save()
	return errors.WithStack(err)
}

// updateNodePullStatuses updates the pull statuses of all nodes.
func (c *Coordinator) updateNodePullStatuses() error {
	c.l.Info("coordinator: updating node pull statuses...")

	needsSync := false
	for _, n := range c.db.Data().Nodes {
		if time.Now().Sub(time.UnixMilli(n.PulledAt)) > time.Minute && n.PullStatus != data.NodeStatusUnavailable {
			c.l.Info(fmt.Sprintf("Node %s marked as unavailable", n.Id))
			n.PullStatus = data.NodeStatusUnavailable
			needsSync = true
		}
	}

	if needsSync {
		err := c.db.Save()
		return errors.WithStack(err)
	}

	return nil
}

// resetUsageForUsers resets the usages of all users.
func (c *Coordinator) resetUsageForUsers() error {
	if c.db.Data().MainSettings.ResetPolicy != "monthly" {
		return nil
	}

	c.l.Info("coordinator: resetting usage for all users...")

	for _, u := range c.db.Data().Users {
		if time.Unix(u.UsageResetAt, 0).Format("2006-01") == time.Now().Format("2006-01") {
			continue
		}

		u.Usage = 0
		u.UsageBytes = 0
		u.Enabled = true
		u.UsageResetAt = time.Now().Unix()
	}

	if err := c.db.Save(); err != nil {
		return errors.WithStack(err)
	}

	go c.UpdateConfigs()

	return nil
}

// State returns the current state of the coordinator.
func (c *Coordinator) State() *State {
	return c.state
}
