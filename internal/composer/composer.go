package composer

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer/socks"
	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/xray"
	xrayConfig "github.com/miladrahimi/p-node/pkg/xray/config"
	"github.com/miladrahimi/p-node/pkg/xray/config/component"
)

// Composer composes the xray config.
type Composer struct {
	config *config.Config
	db     *database.Database[data.Data]
	xray   *xray.Xray
}

// New creates a new composer.
func New(config *config.Config, database *database.Database[data.Data], xray *xray.Xray) *Composer {
	return &Composer{config: config, db: database, xray: xray}
}

// LocalConfig composes the local xray config.
func (c *Composer) LocalConfig(sshConfigsByNodeIds map[string]*ssh.Config) (*xrayConfig.Config, error) {
	clients := c.clients()
	vrClients := c.vrClients()
	d := c.db.Data()
	xs := d.XraySettings
	xc := xrayConfig.New(c.config.Xray.LogLevel)

	apiPort, err := util.FreePort()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	xc.FindInbound("api").Port = apiPort

	hasClients := len(clients) > 0
	hasNodes := len(d.Nodes) > 0
	fallback := &component.VlessFallback{Dest: c.config.HttpServer.Port}

	if hasClients && hasNodes && xs.Vrrv2VrrvPort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
			"relay-vrrv2vrrv",
			xs.Vrrv2VrrvPort,
			xs.VrrvPrivateKey,
			xs.ManagerSni,
			clients,
			fallback,
		))
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"relay-vrrv2vrrv"},
			BalancerTag: "relay-vrrv2vrrv",
		})
		xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
			Tag: "relay-vrrv2vrrv", Selector: []string{},
		})
		for _, n := range d.Nodes {
			outboundRelayPort, err := util.FreePort()
			if err != nil {
				return nil, errors.WithStack(err)
			}
			xc.Outbounds = append(xc.Outbounds, vless.MakeVrrvOutbound(
				fmt.Sprintf("relay-vrrv2vrrv-%s", n.Id),
				n.Host,
				outboundRelayPort,
				util.Uuid(),
				xs.VrrvPublicKey,
				xs.NodeSni,
			))
			xc.FindBalancer("relay-vrrv2vrrv").Selector = append(
				xc.FindBalancer("relay-vrrv2vrrv").Selector,
				fmt.Sprintf("relay-vrrv2vrrv-%s", n.Id),
			)
		}
	}

	if hasClients && hasNodes && xs.Vr2VrrvPort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeVrInbound(
			"relay-vr2vrrv",
			xs.Vr2VrrvPort,
			vrClients,
			fallback,
		))
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"relay-vr2vrrv"},
			BalancerTag: "relay-vr2vrrv",
		})
		xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
			Tag: "relay-vr2vrrv", Selector: []string{},
		})
		for _, n := range d.Nodes {
			outboundRelayPort, err := util.FreePort()
			if err != nil {
				return nil, errors.WithStack(err)
			}
			xc.Outbounds = append(xc.Outbounds, vless.MakeVrrvOutbound(
				fmt.Sprintf("relay-vr2vrrv-%s", n.Id),
				n.Host,
				outboundRelayPort,
				util.Uuid(),
				xs.VrrvPublicKey,
				xs.NodeSni,
			))
			xc.FindBalancer("relay-vr2vrrv").Selector = append(
				xc.FindBalancer("relay-vr2vrrv").Selector,
				fmt.Sprintf("relay-vr2vrrv-%s", n.Id),
			)
		}
	}

	if hasClients && hasNodes && xs.Vrrv2SshPort > 0 {
		outbounds := make([]string, 0, len(d.Nodes))
		for _, n := range d.Nodes {
			sshConfig, ok := sshConfigsByNodeIds[n.Id]
			if !ok || sshConfig.LocalPort <= 0 {
				continue
			}
			tag := fmt.Sprintf("relay-vrrv2ssh-%s", n.Id)
			xc.Outbounds = append(xc.Outbounds, socks.MakeOutbound(tag, "127.0.0.1", sshConfig.LocalPort))
			outbounds = append(outbounds, tag)
		}

		if len(outbounds) > 0 {
			xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
				"relay-vrrv2ssh",
				xs.Vrrv2SshPort,
				xs.VrrvPrivateKey,
				xs.ManagerSni,
				clients,
				fallback,
			))
			xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
				InboundTag:  []string{"relay-vrrv2ssh"},
				BalancerTag: "relay-vrrv2ssh",
			})
			xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
				Tag:      "relay-vrrv2ssh",
				Selector: outbounds,
			})
		}
	}

	if hasClients && hasNodes && xs.Vr2SshPort > 0 {
		outbounds := make([]string, 0, len(d.Nodes))
		for _, n := range d.Nodes {
			sshConfig, ok := sshConfigsByNodeIds[n.Id]
			if !ok || sshConfig.LocalPort <= 0 {
				continue
			}
			tag := fmt.Sprintf("relay-vr2ssh-%s", n.Id)
			xc.Outbounds = append(xc.Outbounds, socks.MakeOutbound(tag, "127.0.0.1", sshConfig.LocalPort))
			outbounds = append(outbounds, tag)
		}

		if len(outbounds) > 0 {
			xc.Inbounds = append(xc.Inbounds, vless.MakeVrInbound(
				"relay-vr2ssh",
				xs.Vr2SshPort,
				vrClients,
				fallback,
			))
			xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
				InboundTag:  []string{"relay-vr2ssh"},
				BalancerTag: "relay-vr2ssh",
			})
			xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
				Tag:      "relay-vr2ssh",
				Selector: outbounds,
			})
		}
	}

	if hasClients && xs.VrrvDirectPort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
			"direct-vrrv",
			xs.VrrvDirectPort,
			xs.VrrvPrivateKey,
			xs.ManagerSni,
			clients,
			fallback,
		))
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"direct-vrrv"},
			OutboundTag: "out",
		})
	}

	return xc, nil
}

// NodeConfig composes the (remote) node xray config.
func (c *Composer) NodeConfig(node *data.Node, lastUpdate time.Time) *xrayConfig.Config {
	d := c.db.Data()
	xs := d.XraySettings
	xc := xrayConfig.New(c.config.Xray.LogLevel)
	fallback := &component.VlessFallback{Dest: node.HttpPort}

	xc.Metadata = &component.Metadata{
		UpdatedAt: lastUpdate.Format(time.RFC3339),
		UpdatedBy: d.MainSettings.Host,
	}

	if xs.Vrrv2VrrvPort > 0 {
		relayOutbound := c.xray.Config().FindOutbound(fmt.Sprintf("relay-vrrv2vrrv-%s", node.Id))
		if relayOutbound != nil && relayOutbound.Settings != nil {
			if len(relayOutbound.Settings.Vnext) > 0 {
				server := relayOutbound.Settings.Vnext[0]
				users := make([]*component.VlessUser, len(server.Users))
				for i, u := range server.Users {
					users[i] = vless.MakeUser(u.Id, vless.FlowVision, vless.EncryptionEmpty)
				}
				xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
					"relay-vrrv2vrrv",
					server.Port,
					xs.VrrvPrivateKey,
					xs.NodeSni,
					users,
					fallback,
				))
				xc.Routing.Rules = append(
					xc.Routing.Rules,
					&component.Rule{
						InboundTag:  []string{"relay-vrrv2vrrv"},
						OutboundTag: "out",
					},
				)
			}
		}
	}

	if xs.Vr2VrrvPort > 0 {
		relayOutbound := c.xray.Config().FindOutbound(fmt.Sprintf("relay-vr2vrrv-%s", node.Id))
		if relayOutbound != nil && relayOutbound.Settings != nil {
			if len(relayOutbound.Settings.Vnext) > 0 {
				server := relayOutbound.Settings.Vnext[0]
				users := make([]*component.VlessUser, len(server.Users))
				for i, u := range server.Users {
					users[i] = vless.MakeUser(u.Id, vless.FlowVision, vless.EncryptionEmpty)
				}
				xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
					"relay-vr2vrrv",
					server.Port,
					xs.VrrvPrivateKey,
					xs.NodeSni,
					users,
					fallback,
				))
				xc.Routing.Rules = append(
					xc.Routing.Rules,
					&component.Rule{
						InboundTag:  []string{"relay-vr2vrrv"},
						OutboundTag: "out",
					},
				)
			}
		}
	}

	if xs.VrrvRemotePort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
			"remote-vrrv",
			xs.VrrvRemotePort,
			xs.VrrvPrivateKey,
			xs.NodeSni,
			c.clients(),
			fallback,
		))
		xc.Routing.Rules = append(
			xc.Routing.Rules,
			&component.Rule{
				InboundTag:  []string{"remote-vrrv"},
				OutboundTag: "out",
			},
		)
	}

	return xc
}

// clients returns the list of clients from the database users.
func (c *Composer) clients() []*component.VlessUser {
	var clients []*component.VlessUser
	for _, u := range c.db.Data().Users {
		if !u.Enabled {
			continue
		}
		clients = append(clients, vless.MakeUser(u.VlessId, vless.FlowVision, vless.EncryptionEmpty))
	}
	return clients
}

// vrClients returns VLESS clients without flow settings.
func (c *Composer) vrClients() []*component.VlessUser {
	var clients []*component.VlessUser
	for _, u := range c.db.Data().Users {
		if !u.Enabled {
			continue
		}
		clients = append(clients, vless.MakeUser(u.VlessId, "", vless.EncryptionEmpty))
	}
	return clients
}
