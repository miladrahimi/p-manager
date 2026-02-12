package composer

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/data"
)

// UserLinks builds all available proxy links for a user.
func (c *Composer) UserLinks(user *data.User) map[string]string {
	d := c.db.Data()
	xs := d.XraySettings
	links := make(map[string]string)

	addLink := func(name, host string, port int, sni string) {
		if host == "" || port <= 0 {
			return
		}
		links[name] = buildRrLink(host, port, user.VlessId, xs.RrPublicKey, sni, name)
	}

	addLink("rr_direct", d.MainSettings.Host, xs.RrDirectPort, xs.ManagerSni)
	addLink("rr_2_rr_relay", d.MainSettings.Host, xs.Rr2RrPort, xs.ManagerSni)
	addLink("rr_2_ssh", d.MainSettings.Host, xs.Rr2SshPort, xs.ManagerSni)

	if xs.RrRemotePort > 0 {
		for _, n := range d.Nodes {
			name := fmt.Sprintf("rr_remote_%s", strings.ReplaceAll(n.Host, ".", "_"))
			addLink(name, n.Host, xs.RrRemotePort, xs.NodeSni)
		}
	}

	return links
}

func buildRrLink(host string, port int, userId string, publicKey string, nodeSni string, tag string) string {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	vlessUrl := url.URL{
		Scheme:   vless.Protocol,
		User:     url.User(userId),
		Host:     address,
		Fragment: tag,
	}

	query := url.Values{}
	query.Set("flow", vless.FlowVision)
	query.Set("encryption", vless.EncryptionNone)
	query.Set("type", vless.NetworkRaw)
	query.Set("security", vless.SecurityReality)
	query.Set("sni", nodeSni)
	query.Set("pbk", publicKey)

	vlessUrl.RawQuery = query.Encode()

	return vlessUrl.String()
}
