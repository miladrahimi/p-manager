package composer

import (
	"fmt"

	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/data"
)

// AccountLinks builds all available proxy links for an account.
func (c *Composer) AccountLinks(account *data.Account) map[string]string {
	links := make(map[string]string)
	c.db.Read(func(d *data.Data) {
		c.accountLinks(d, account, links)
	})
	return links
}

// accountLinks fills links with the account's proxy links. The caller must hold
// the store read lock.
func (c *Composer) accountLinks(d *data.Data, account *data.Account, links map[string]string) {
	xs := d.XraySettings

	addRrLink := func(name, host string, port int, sni string) {
		if host == "" || port <= 0 {
			return
		}
		nameWithHost := fmt.Sprintf("%s@%s", name, d.MainSettings.Host)
		links[nameWithHost] = makeVlessLink(vlessLinkOptions{
			host:      host,
			port:      port,
			userId:    account.ProxyId,
			tag:       nameWithHost,
			flow:      vless.FlowVision,
			network:   vless.NetworkRaw,
			security:  vless.SecurityReality,
			sni:       sni,
			publicKey: xs.RealityPublicKey,
		})
	}

	addRrLink("direct-rr", d.MainSettings.Host, xs.DirectRrPort, xs.ManagerSni)
	addRrLink("relay-rr2rr", d.MainSettings.Host, xs.RelayRr2RrManagerPort, xs.ManagerSni)
	addRrLink("relay-rr2ssh", d.MainSettings.Host, xs.RelayRr2SshPort, xs.ManagerSni)

	if xs.RemoteRrPort > 0 {
		for _, n := range d.Nodes {
			name := fmt.Sprintf("remote-rr-%s", n.Host)
			addRrLink(name, n.Host, xs.RemoteRrPort, xs.NodeSni)
		}
	}
}
