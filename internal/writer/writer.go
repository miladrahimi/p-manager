package writer

import (
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/internal/utils"
	"github.com/miladrahimi/p-node/pkg/xray"
	"strconv"
	"time"
)

type Writer struct {
	c        *config.Config
	hc       *client.Client
	database *database.Database
	xray     *xray.Xray
}

func (w *Writer) clients() []*xray.Client {
	var clients []*xray.Client
	for _, u := range w.database.Content.Users {
		if !u.Enabled {
			continue
		}
		clients = append(clients, &xray.Client{
			Email:    strconv.Itoa(u.Id),
			Password: u.ShadowsocksPassword,
			Method:   u.ShadowsocksMethod,
		})
	}
	return clients
}

func (w *Writer) LocalConfig() (*xray.Config, error) {
	clients := w.clients()

	apiPort, err := utils.FreePort()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	xc := xray.NewConfig(w.c.Xray.LogLevel)

	xc.FindInbound("api").Port = apiPort

	var key string
	if len(clients) > 0 {
		if w.database.Content.Settings.SsRelayPort > 0 {
			if key, err = utils.Key32(); err != nil {
				return nil, err
			}
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"relay",
				key,
				config.ShadowsocksMethod,
				"tcp,udp",
				w.database.Content.Settings.SsRelayPort,
				clients,
			))
		}
		if w.database.Content.Settings.SsReversePort > 0 {
			if key, err = utils.Key32(); err != nil {
				return nil, err
			}
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"reverse",
				key,
				config.ShadowsocksMethod,
				"tcp,udp",
				w.database.Content.Settings.SsReversePort,
				clients,
			))
		}
		if w.database.Content.Settings.SsDirectPort > 0 {
			if key, err = utils.Key32(); err != nil {
				return nil, err
			}
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"direct",
				key,
				config.ShadowsocksMethod,
				"tcp,udp",
				w.database.Content.Settings.SsDirectPort,
				clients,
			))
		}
	}

	if len(clients) > 0 {
		if w.database.Content.Settings.SsDirectPort > 0 {
			xc.Routing.Rules = append(xc.Routing.Rules, &xray.Rule{
				InboundTag:  []string{"direct"},
				OutboundTag: "out",
			})
		}
		if len(w.database.Content.Nodes) > 0 {
			if w.database.Content.Settings.SsRelayPort > 0 {
				xc.Routing.Rules = append(xc.Routing.Rules, &xray.Rule{
					InboundTag:  []string{"relay"},
					BalancerTag: "relay",
				})
			}
			if w.database.Content.Settings.SsReversePort > 0 {
				xc.Routing.Rules = append(xc.Routing.Rules, &xray.Rule{
					InboundTag:  []string{"reverse"},
					BalancerTag: "portal",
				})
			}
		}
	}

	if len(w.database.Content.Nodes) > 0 {
		if w.database.Content.Settings.SsRelayPort > 0 {
			xc.Routing.Balancers = append(xc.Routing.Balancers, &xray.Balancer{Tag: "relay", Selector: []string{}})
		}
		if w.database.Content.Settings.SsReversePort > 0 {
			xc.Routing.Balancers = append(xc.Routing.Balancers, &xray.Balancer{Tag: "portal", Selector: []string{}})
		}
	}

	for _, s := range w.database.Content.Nodes {
		inboundPort, err := utils.FreePort()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if w.database.Content.Settings.SsReversePort > 0 {
			if key, err = utils.Key32(); err != nil {
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
			xc.Reverse.Portals = append(xc.Reverse.Portals, &xray.ReverseItem{
				Tag:    fmt.Sprintf("portal-%d", s.Id),
				Domain: fmt.Sprintf("s%d.reverse.proxy", s.Id),
			})
			xc.Routing.Rules = append(xc.Routing.Rules, &xray.Rule{
				InboundTag:  []string{fmt.Sprintf("internal-%d", s.Id)},
				OutboundTag: fmt.Sprintf("portal-%d", s.Id),
			})
			xc.FindBalancer("portal").Selector = append(
				xc.FindBalancer("portal").Selector,
				fmt.Sprintf("portal-%d", s.Id),
			)
		}

		if w.database.Content.Settings.SsRelayPort > 0 {
			outboundRelayPort, err := utils.FreePort()
			if err != nil {
				return nil, errors.WithStack(err)
			}
			if key, err = utils.Key32(); err != nil {
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

func (w *Writer) RemoteConfig(node *database.Node, lastUpdate time.Time, password string) *xray.Config {
	xc := xray.NewConfig(w.c.Xray.LogLevel)

	xc.Metadata = &xray.Metadata{
		UpdatedAt: lastUpdate.Format(time.RFC3339),
		UpdatedBy: w.database.Content.Settings.Host,
	}

	if w.database.Content.Settings.SsRelayPort > 0 {
		relayOutbound := w.xray.Config().FindOutbound(fmt.Sprintf("relay-%d", node.Id))
		xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
			"direct",
			relayOutbound.Settings.Servers[0].Password,
			relayOutbound.Settings.Servers[0].Method,
			"tcp",
			relayOutbound.Settings.Servers[0].Port,
			nil,
		))
		xc.Routing.Rules = append(
			xc.Routing.Rules,
			&xray.Rule{
				InboundTag:  []string{"direct"},
				OutboundTag: "out",
			},
		)
	}

	if w.database.Content.Settings.SsReversePort > 0 {
		internalOutbound := w.xray.Config().FindInbound(fmt.Sprintf("internal-%d", node.Id))
		xc.Outbounds = append(xc.Outbounds, xc.MakeShadowsocksOutbound(
			"internal",
			w.database.Content.Settings.Host,
			internalOutbound.Settings.Password,
			internalOutbound.Settings.Method,
			internalOutbound.Port,
		))
		xc.Reverse.Bridges = append(xc.Reverse.Bridges, &xray.ReverseItem{
			Tag:    "bridge",
			Domain: fmt.Sprintf("s%d.reverse.proxy", node.Id),
		})
		xc.Routing.Rules = append(
			xc.Routing.Rules,
			&xray.Rule{
				InboundTag:  []string{"bridge"},
				Domain:      []string{fmt.Sprintf("full:s%d.reverse.proxy", node.Id)},
				OutboundTag: "internal",
			},
			&xray.Rule{
				InboundTag:  []string{"bridge"},
				OutboundTag: "out",
			},
		)
	}

	if w.database.Content.Settings.SsRemotePort > 0 {
		xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
			"remote",
			password,
			config.ShadowsocksMethod,
			"tcp",
			w.database.Content.Settings.SsRemotePort,
			w.clients(),
		))
		xc.Routing.Rules = append(
			xc.Routing.Rules,
			&xray.Rule{
				InboundTag:  []string{"remote"},
				OutboundTag: "out",
			},
		)
	}

	return xc
}

func New(config *config.Config, database *database.Database, xray *xray.Xray) *Writer {
	return &Writer{c: config, database: database, xray: xray}
}
