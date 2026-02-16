package composer

import (
	"fmt"

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
		tag := name
		tag = fmt.Sprintf("%s@%s", name, d.MainSettings.Host)
		links[name] = buildRrLink(host, port, user.VlessId, xs.RrPublicKey, sni, tag)
	}

	addLink("direct_rr", d.MainSettings.Host, xs.DirectRrPort, xs.ManagerSni)
	addLink("relay_rr_2_rr", d.MainSettings.Host, xs.RelayRr2RrPort, xs.ManagerSni)
	addLink("relay_rr_2_ssh", d.MainSettings.Host, xs.RelayRr2SshPort, xs.ManagerSni)

	if xs.RemoteRrPort > 0 {
		for _, n := range d.Nodes {
			name := fmt.Sprintf("remote_rr_%s", n.Host)
			addLink(name, n.Host, xs.RemoteRrPort, xs.NodeSni)
		}
	}

	return links
}
