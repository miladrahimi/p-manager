package composer

import (
	"fmt"

	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/data"
)

// UserLinks builds all available proxy links for a user.
func (c *Composer) UserLinks(user *data.User) map[string]string {
	d := c.db.Data()
	xs := d.XraySettings
	links := make(map[string]string)

	addRrLink := func(name, host string, port int, sni string) {
		if host == "" || port <= 0 {
			return
		}
		nameWithHost := fmt.Sprintf("%s@%s", name, d.MainSettings.Host)
		links[nameWithHost] = makeVlessLink(vlessLinkOptions{
			host:      host,
			port:      port,
			userId:    user.ProxyId,
			tag:       nameWithHost,
			flow:      vless.FlowVision,
			network:   vless.NetworkRaw,
			security:  vless.SecurityReality,
			sni:       sni,
			publicKey: xs.RealityPublicKey,
		})
	}

	addRrLink("direct-rr", d.MainSettings.Host, xs.DirectRrPort, xs.ManagerSni)
	addRrLink("relay-rr2rr", d.MainSettings.Host, xs.RelayRr2RrPort, xs.ManagerSni)
	addRrLink("relay-rr2ssh", d.MainSettings.Host, xs.RelayRr2SshPort, xs.ManagerSni)

	if xs.RemoteRrPort > 0 {
		for _, n := range d.Nodes {
			name := fmt.Sprintf("remote-rr-%s", n.Host)
			addRrLink(name, n.Host, xs.RemoteRrPort, xs.NodeSni)
		}
	}

	return links
}
