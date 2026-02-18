package composer

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer/socks"
	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-manager/pkg/util"
	xrayConfig "github.com/miladrahimi/p-node/pkg/xray/config"
	"github.com/miladrahimi/p-node/pkg/xray/config/component"
)

// ManagerConfig composes the P-Manager (local) xray config.
func (c *Composer) ManagerConfig(sshConfigsByNodeIds map[string][]*ssh.ProxyConfig) (*xrayConfig.Config, error) {
	rrClients := c.rrClients()
	d := c.db.Data()
	xs := d.XraySettings
	xc := xrayConfig.New(c.config.Xray.LogLevel)

	apiPort, err := util.FreePort()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	xc.FindInbound("api").Port = apiPort

	hasClients := len(rrClients) > 0
	hasNodes := len(d.Nodes) > 0
	fallback := &component.Fallback{Dest: c.config.HttpServer.Port}

	if hasClients && hasNodes && xs.RelayRr2RrManagerPort > 0 && xs.RelayRr2RrNodePort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeRrInbound(
			"relay-rr2rr",
			xs.RelayRr2RrManagerPort,
			xs.RealityPrivateKey,
			xs.ManagerSni,
			rrClients,
			fallback,
		))
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"relay-rr2rr"},
			BalancerTag: "relay-rr2rr",
		})
		xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
			Tag: "relay-rr2rr", Selector: []string{},
		})
		for _, n := range d.Nodes {
			xc.Outbounds = append(xc.Outbounds, vless.MakeRrOutbound(
				fmt.Sprintf("relay-rr2rr-%s", n.Id),
				n.Host,
				xs.RelayRr2RrNodePort,
				util.Uuid(),
				xs.RealityPublicKey,
				xs.NodeSni,
			))
			xc.FindBalancer("relay-rr2rr").Selector = append(
				xc.FindBalancer("relay-rr2rr").Selector,
				fmt.Sprintf("relay-rr2rr-%s", n.Id),
			)
		}
	}

	if hasClients && hasNodes && xs.RelayRr2SshPort > 0 {
		outbounds := make([]string, 0, len(d.Nodes))
		for _, n := range d.Nodes {
			proxyConfigs, ok := sshConfigsByNodeIds[n.Id]
			if !ok || len(proxyConfigs) == 0 {
				continue
			}
			for i, proxyConfig := range proxyConfigs {
				if proxyConfig == nil || proxyConfig.LocalPort <= 0 {
					continue
				}
				tag := fmt.Sprintf("relay-rr2ssh-%s-%d", n.Id, i+1)
				xc.Outbounds = append(xc.Outbounds, socks.MakeOutbound(tag, "127.0.0.1", proxyConfig.LocalPort))
				outbounds = append(outbounds, tag)
			}
		}

		if len(outbounds) > 0 {
			xc.Inbounds = append(xc.Inbounds, vless.MakeRrInbound(
				"relay-rr2ssh",
				xs.RelayRr2SshPort,
				xs.RealityPrivateKey,
				xs.ManagerSni,
				rrClients,
				fallback,
			))
			xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
				InboundTag:  []string{"relay-rr2ssh"},
				BalancerTag: "relay-rr2ssh",
			})
			xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
				Tag:      "relay-rr2ssh",
				Selector: outbounds,
			})
		}
	}

	if hasClients && xs.DirectRrPort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeRrInbound(
			"direct-rr",
			xs.DirectRrPort,
			xs.RealityPrivateKey,
			xs.ManagerSni,
			rrClients,
			fallback,
		))
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"direct-rr"},
			OutboundTag: "out",
		})
	}

	return xc, nil
}

// NodeConfig composes the (remote) P-Node xray config.
func (c *Composer) NodeConfig(node *data.Node, lastUpdate time.Time) *xrayConfig.Config {
	d := c.db.Data()
	xs := d.XraySettings
	xc := xrayConfig.New(c.config.Xray.LogLevel)
	fallback := &component.Fallback{Dest: node.HttpPort}

	xc.Metadata = &component.Metadata{
		UpdatedAt: lastUpdate.Format(time.RFC3339),
		UpdatedBy: d.MainSettings.Host,
	}

	if xs.RelayRr2RrNodePort > 0 {
		relayOutbound := c.xray.Config().FindOutbound(fmt.Sprintf("relay-rr2rr-%s", node.Id))
		if relayOutbound != nil && relayOutbound.Settings != nil {
			if len(relayOutbound.Settings.Vnext) > 0 {
				server := relayOutbound.Settings.Vnext[0]
				clients := make([]*component.Client, len(server.Users))
				for i, u := range server.Users {
					if u == nil {
						continue
					}
					copyUser := *u
					copyUser.Encryption = vless.EncryptionEmpty
					clients[i] = &copyUser
				}
				xc.Inbounds = append(xc.Inbounds, vless.MakeRrInbound(
					"relay-rr2rr",
					xs.RelayRr2RrNodePort,
					xs.RealityPrivateKey,
					xs.NodeSni,
					clients,
					fallback,
				))
				xc.Routing.Rules = append(
					xc.Routing.Rules,
					&component.Rule{
						InboundTag:  []string{"relay-rr2rr"},
						OutboundTag: "out",
					},
				)
			}
		}
	}

	if xs.RemoteRrPort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeRrInbound(
			"remote-rr",
			xs.RemoteRrPort,
			xs.RealityPrivateKey,
			xs.NodeSni,
			c.rrClients(),
			fallback,
		))
		xc.Routing.Rules = append(
			xc.Routing.Rules,
			&component.Rule{
				InboundTag:  []string{"remote-rr"},
				OutboundTag: "out",
			},
		)
	}

	return xc
}

// rrClients returns the RR-ready client list.
func (c *Composer) rrClients() []*component.Client {
	var clients []*component.Client
	for _, u := range c.db.Data().Accounts {
		if !u.Enabled {
			continue
		}
		clients = append(clients, vless.MakeUser(u.ProxyId, vless.FlowVision, vless.EncryptionEmpty))
	}
	return clients
}
