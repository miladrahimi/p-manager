package composer

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer/socks"
	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
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
func (c *Composer) LocalConfig(sshLocalPorts map[string]int) (*xrayConfig.Config, error) {
	clients := c.clients()
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
			"relay",
			xs.Vrrv2VrrvPort,
			xs.VrrvPrivateKey,
			xs.ManagerSni,
			clients,
			fallback,
		))
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"relay"},
			BalancerTag: "relay",
		})
		xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
			Tag: "relay", Selector: []string{},
		})
		for _, n := range d.Nodes {
			outboundRelayPort, err := util.FreePort()
			if err != nil {
				return nil, errors.WithStack(err)
			}
			xc.Outbounds = append(xc.Outbounds, vless.MakeVrrvOutbound(
				fmt.Sprintf("relay-%s", n.Id),
				n.Host,
				outboundRelayPort,
				util.Uuid(),
				xs.VrrvPublicKey,
				xs.NodeSni,
			))
			xc.FindBalancer("relay").Selector = append(
				xc.FindBalancer("relay").Selector,
				fmt.Sprintf("relay-%s", n.Id),
			)
		}
	}

	if hasClients && hasNodes && xs.Vrrv2SshPort > 0 {
		outbounds := make([]string, 0, len(d.Nodes))
		for _, n := range d.Nodes {
			localPort, ok := sshLocalPorts[n.Id]
			if !ok || localPort <= 0 {
				continue
			}
			tag := fmt.Sprintf("vrrv2ssh-%s", n.Id)
			xc.Outbounds = append(xc.Outbounds, socks.MakeOutbound(tag, "127.0.0.1", localPort))
			outbounds = append(outbounds, tag)
		}

		if len(outbounds) > 0 {
			xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
				"vrrv2ssh",
				xs.Vrrv2SshPort,
				xs.VrrvPrivateKey,
				xs.ManagerSni,
				clients,
				fallback,
			))
			xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
				InboundTag:  []string{"vrrv2ssh"},
				BalancerTag: "vrrv2ssh",
			})
			xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
				Tag:      "vrrv2ssh",
				Selector: outbounds,
			})
		}
	}

	if hasClients && xs.VrrvDirectPort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
			"direct",
			xs.VrrvDirectPort,
			xs.VrrvPrivateKey,
			xs.ManagerSni,
			clients,
			fallback,
		))
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"direct"},
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
		relayOutbound := c.xray.Config().FindOutbound(fmt.Sprintf("relay-%s", node.Id))
		if relayOutbound != nil && relayOutbound.Settings != nil {
			if len(relayOutbound.Settings.Vnext) > 0 {
				server := relayOutbound.Settings.Vnext[0]
				users := make([]*component.VlessUser, len(server.Users))
				for i, u := range server.Users {
					users[i] = vless.MakeUser(u.Id, vless.FlowVision, vless.EncryptionEmpty)
				}
				xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
					"relay",
					server.Port,
					xs.VrrvPrivateKey,
					xs.NodeSni,
					users,
					fallback,
				))
				xc.Routing.Rules = append(
					xc.Routing.Rules,
					&component.Rule{
						InboundTag:  []string{"relay"},
						OutboundTag: "out",
					},
				)
			}
		}
	}

	if xs.VrrvRemotePort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
			"remote",
			xs.VrrvRemotePort,
			xs.VrrvPrivateKey,
			xs.NodeSni,
			c.clients(),
			fallback,
		))
		xc.Routing.Rules = append(
			xc.Routing.Rules,
			&component.Rule{
				InboundTag:  []string{"remote"},
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
