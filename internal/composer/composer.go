package composer

import (
	"fmt"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/xray"
	xrayConfig "github.com/miladrahimi/p-node/pkg/xray/config"
	xrayComponent "github.com/miladrahimi/p-node/pkg/xray/config/component"
	xrayProtocol "github.com/miladrahimi/p-node/pkg/xray/config/protocol"
)

// Composer composes the xray config.
type Composer struct {
	c        *config.Config
	hc       *client.Client
	database *database.Database[data.Data]
	xray     *xray.Xray
}

// New creates a new composer.
func New(config *config.Config, database *database.Database[data.Data], xray *xray.Xray) *Composer {
	return &Composer{c: config, database: database, xray: xray}
}

// LocalConfig composes the local xray config.
func (w *Composer) LocalConfig() (*xrayConfig.Config, error) {
	clients := w.clients()

	apiPort, err := util.FreePort()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	xc := xrayConfig.New(w.c.Xray.LogLevel)

	xc.FindInbound("api").Port = apiPort

	var key string
	if len(clients) > 0 {
		if w.database.Data().Settings.SsRelayPort > 0 {
			if key, err = util.Key32(); err != nil {
				return nil, err
			}
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"relay",
				key,
				config.ShadowsocksMethod,
				"tcp,udp",
				w.database.Data().Settings.SsRelayPort,
				clients,
			))
		}
		if w.database.Data().Settings.SsReversePort > 0 {
			if key, err = util.Key32(); err != nil {
				return nil, err
			}
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"reverse",
				key,
				config.ShadowsocksMethod,
				"tcp,udp",
				w.database.Data().Settings.SsReversePort,
				clients,
			))
		}
		if w.database.Data().Settings.SsDirectPort > 0 {
			if key, err = util.Key32(); err != nil {
				return nil, err
			}
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"direct",
				key,
				config.ShadowsocksMethod,
				"tcp,udp",
				w.database.Data().Settings.SsDirectPort,
				clients,
			))
		}
	}

	if len(clients) > 0 {
		if w.database.Data().Settings.SsDirectPort > 0 {
			xc.Routing.Rules = append(xc.Routing.Rules, &xrayComponent.Rule{
				InboundTag:  []string{"direct"},
				OutboundTag: "out",
			})
		}
		if len(w.database.Data().Nodes) > 0 {
			if w.database.Data().Settings.SsRelayPort > 0 {
				xc.Routing.Rules = append(xc.Routing.Rules, &xrayComponent.Rule{
					InboundTag:  []string{"relay"},
					BalancerTag: "relay",
				})
			}
			if w.database.Data().Settings.SsReversePort > 0 {
				xc.Routing.Rules = append(xc.Routing.Rules, &xrayComponent.Rule{
					InboundTag:  []string{"reverse"},
					BalancerTag: "portal",
				})
			}
		}
	}

	if len(w.database.Data().Nodes) > 0 {
		if w.database.Data().Settings.SsRelayPort > 0 {
			xc.Routing.Balancers = append(xc.Routing.Balancers, &xrayComponent.Balancer{Tag: "relay", Selector: []string{}})
		}
		if w.database.Data().Settings.SsReversePort > 0 {
			xc.Routing.Balancers = append(xc.Routing.Balancers, &xrayComponent.Balancer{Tag: "portal", Selector: []string{}})
		}
	}

	for _, s := range w.database.Data().Nodes {
		inboundPort, err := util.FreePort()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if w.database.Data().Settings.SsReversePort > 0 {
			if key, err = util.Key32(); err != nil {
				return nil, err
			}
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				fmt.Sprintf("internal-%d", s.Id),
				key,
				config.Shadowsocks2022Method,
				"tcp",
				inboundPort,
				nil,
			))
			xc.Reverse.Portals = append(xc.Reverse.Portals, &xrayComponent.ReverseItem{
				Tag:    fmt.Sprintf("portal-%d", s.Id),
				Domain: fmt.Sprintf("s%d.reverse.proxy", s.Id),
			})
			xc.Routing.Rules = append(xc.Routing.Rules, &xrayComponent.Rule{
				InboundTag:  []string{fmt.Sprintf("internal-%d", s.Id)},
				OutboundTag: fmt.Sprintf("portal-%d", s.Id),
			})
			xc.FindBalancer("portal").Selector = append(
				xc.FindBalancer("portal").Selector,
				fmt.Sprintf("portal-%d", s.Id),
			)
		}

		if w.database.Data().Settings.SsRelayPort > 0 {
			outboundRelayPort, err := util.FreePort()
			if err != nil {
				return nil, errors.WithStack(err)
			}
			if key, err = util.Key32(); err != nil {
				return nil, err
			}
			xc.Outbounds = append(xc.Outbounds, xc.MakeShadowsocksOutbound(
				fmt.Sprintf("relay-%d", s.Id),
				s.Host,
				key,
				config.Shadowsocks2022Method,
				outboundRelayPort,
			))
			xc.FindBalancer("relay").Selector = append(
				xc.FindBalancer("relay").Selector,
				fmt.Sprintf("relay-%d", s.Id),
			)
		}
	}

	return xc, nil
}

// NodeConfig composes the (remote) node xray config.
func (w *Composer) NodeConfig(node *data.Node, lastUpdate time.Time, password string) *xrayConfig.Config {
	xc := xrayConfig.New(w.c.Xray.LogLevel)

	xc.Metadata = &xrayComponent.Metadata{
		UpdatedAt: lastUpdate.Format(time.RFC3339),
		UpdatedBy: w.database.Data().Settings.Host,
	}

	if w.database.Data().Settings.SsRelayPort > 0 {
		relayOutbound := w.xray.Config().FindOutbound(fmt.Sprintf("relay-%d", node.Id))
		if relayOutbound != nil {
			settings, ok := relayOutbound.Settings.(*xrayProtocol.SsOutboundSettings)
			if ok && len(settings.Servers) > 0 {
				server := settings.Servers[0]
				xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
					"direct",
					server.Password,
					server.Method,
					"tcp",
					server.Port,
					nil,
				))
				xc.Routing.Rules = append(
					xc.Routing.Rules,
					&xrayComponent.Rule{
						InboundTag:  []string{"direct"},
						OutboundTag: "out",
					},
				)
			}
		}
	}

	if w.database.Data().Settings.SsReversePort > 0 {
		internalOutbound := w.xray.Config().FindInbound(fmt.Sprintf("internal-%d", node.Id))
		if internalOutbound != nil {
			settings, ok := internalOutbound.Settings.(*xrayComponent.InboundSettings)
			if ok {
				xc.Outbounds = append(xc.Outbounds, xc.MakeShadowsocksOutbound(
					"internal",
					w.database.Data().Settings.Host,
					settings.Password,
					settings.Method,
					internalOutbound.Port,
				))
				xc.Reverse.Bridges = append(xc.Reverse.Bridges, &xrayComponent.ReverseItem{
					Tag:    "bridge",
					Domain: fmt.Sprintf("s%d.reverse.proxy", node.Id),
				})
				xc.Routing.Rules = append(
					xc.Routing.Rules,
					&xrayComponent.Rule{
						InboundTag:  []string{"bridge"},
						Domain:      []string{fmt.Sprintf("full:s%d.reverse.proxy", node.Id)},
						OutboundTag: "internal",
					},
					&xrayComponent.Rule{
						InboundTag:  []string{"bridge"},
						OutboundTag: "out",
					},
				)
			}
		}
	}

	if w.database.Data().Settings.SsRemotePort > 0 {
		xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
			"remote",
			password,
			config.ShadowsocksMethod,
			"tcp",
			w.database.Data().Settings.SsRemotePort,
			w.clients(),
		))
		xc.Routing.Rules = append(
			xc.Routing.Rules,
			&xrayComponent.Rule{
				InboundTag:  []string{"remote"},
				OutboundTag: "out",
			},
		)
	}

	return xc
}

// clients returns the list of clients from the database.
func (w *Composer) clients() []*xrayProtocol.SsClient {
	var clients []*xrayProtocol.SsClient
	for _, u := range w.database.Data().Users {
		if !u.Enabled {
			continue
		}
		clients = append(clients, &xrayProtocol.SsClient{
			Email:    strconv.Itoa(u.Id),
			Password: u.ShadowsocksPassword,
			Method:   u.ShadowsocksMethod,
		})
	}
	return clients
}
