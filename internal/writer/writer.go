package writer

import (
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/internal/utils"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
	"go.uber.org/zap"
	"strconv"
)

type Writer struct {
	l        *logger.Logger
	hc       *client.Client
	database *database.Database
	xray     *xray.Xray
}

func (c *Writer) clients() []*xray.Client {
	var clients []*xray.Client
	for _, u := range c.database.Data.Users {
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

func (c *Writer) LocalConfig() *xray.Config {
	clients := c.clients()

	apiPort, err := utils.FreePort()
	if err != nil {
		c.l.Fatal("writer: cannot find port for xray api", zap.Error(errors.WithStack(err)))
	}

	xc := xray.NewConfig()
	xc.FindInbound("api").Port = apiPort

	if len(clients) > 0 {
		if c.database.Data.Settings.SsRelayPort > 0 {
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"relay",
				utils.Key32(),
				config.ShadowsocksMethod,
				c.database.Data.Settings.SsRelayPort,
				clients,
			))
		}
		if c.database.Data.Settings.SsReversePort > 0 {
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"reverse",
				utils.Key32(),
				config.ShadowsocksMethod,
				c.database.Data.Settings.SsReversePort,
				clients,
			))
		}
		if c.database.Data.Settings.SsDirectPort > 0 {
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"direct",
				utils.Key32(),
				config.ShadowsocksMethod,
				c.database.Data.Settings.SsDirectPort,
				clients,
			))
		}
	}

	if len(clients) > 0 {
		if c.database.Data.Settings.SsDirectPort > 0 {
			xc.Routing.Settings.Rules = append(xc.Routing.Settings.Rules, &xray.Rule{
				InboundTag:  []string{"direct"},
				OutboundTag: "freedom",
				Type:        "field",
			})
		}
		if len(c.database.Data.Servers) > 0 {
			if c.database.Data.Settings.SsRelayPort > 0 {
				xc.Routing.Settings.Rules = append(xc.Routing.Settings.Rules, &xray.Rule{
					InboundTag:  []string{"relay"},
					BalancerTag: "relay",
					Type:        "field",
				})
			}
			if c.database.Data.Settings.SsReversePort > 0 {
				xc.Routing.Settings.Rules = append(xc.Routing.Settings.Rules, &xray.Rule{
					InboundTag:  []string{"reverse"},
					BalancerTag: "portal",
					Type:        "field",
				})
			}
		}
	}

	if len(c.database.Data.Servers) > 0 {
		if c.database.Data.Settings.SsRelayPort > 0 {
			xc.Routing.Balancers = append(xc.Routing.Balancers, &xray.Balancer{Tag: "relay", Selector: []string{}})
		}
		if c.database.Data.Settings.SsReversePort > 0 {
			xc.Routing.Balancers = append(xc.Routing.Balancers, &xray.Balancer{Tag: "portal", Selector: []string{}})
		}
	}

	for _, s := range c.database.Data.Servers {
		inboundPort, err := utils.FreePort()
		if err != nil {
			c.l.Fatal("writer: cannot find port for foreign inbound", zap.Error(errors.WithStack(err)))
		}

		if c.database.Data.Settings.SsReversePort > 0 {
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				fmt.Sprintf("foreign-%d", s.Id),
				utils.Key32(),
				config.Shadowsocks2022Method,
				inboundPort,
				nil,
			))
			xc.Reverse.Portals = append(xc.Reverse.Portals, &xray.ReverseItem{
				Tag:    fmt.Sprintf("portal-%d", s.Id),
				Domain: fmt.Sprintf("s%d.google.com", s.Id),
			})
			xc.Routing.Settings.Rules = append(xc.Routing.Settings.Rules, &xray.Rule{
				InboundTag:  []string{fmt.Sprintf("foreign-%d", s.Id)},
				OutboundTag: fmt.Sprintf("portal-%d", s.Id),
				Type:        "field",
			})
			xc.FindBalancer("portal").Selector = append(
				xc.FindBalancer("portal").Selector,
				fmt.Sprintf("portal-%d", s.Id),
			)
		}

		if c.database.Data.Settings.SsRelayPort > 0 {
			outboundRelayPort, err := utils.FreePort()
			if err != nil {
				c.l.Fatal("writer: cannot find port for relay outbound", zap.Error(errors.WithStack(err)))
			}
			xc.Outbounds = append(xc.Outbounds, xc.MakeShadowsocksOutbound(
				fmt.Sprintf("relay-%d", s.Id),
				s.Host,
				utils.Key32(),
				config.Shadowsocks2022Method,
				outboundRelayPort,
			))
			xc.FindBalancer("relay").Selector = append(
				xc.FindBalancer("relay").Selector,
				fmt.Sprintf("relay-%d", s.Id),
			)
		}
	}

	return xc
}

func (c *Writer) RemoteConfig(s *database.Server) *xray.Config {
	xc := xray.NewConfig()

	if c.database.Data.Settings.SsRelayPort > 0 {
		relayOutbound := c.xray.Config().FindOutbound(fmt.Sprintf("relay-%d", s.Id))
		xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
			"direct",
			relayOutbound.Settings.Servers[0].Password,
			relayOutbound.Settings.Servers[0].Method,
			relayOutbound.Settings.Servers[0].Port,
			nil,
		))
		xc.Routing.Settings.Rules = append(
			xc.Routing.Settings.Rules,
			&xray.Rule{
				Type:        "field",
				InboundTag:  []string{"direct"},
				OutboundTag: "freedom",
			},
		)
	}

	if c.database.Data.Settings.SsReversePort > 0 {
		foreignOutbound := c.xray.Config().FindInbound(fmt.Sprintf("foreign-%d", s.Id))
		xc.Outbounds = append(xc.Outbounds, xc.MakeShadowsocksOutbound(
			"foreign",
			c.database.Data.Settings.Host,
			foreignOutbound.Settings.Password,
			foreignOutbound.Settings.Method,
			foreignOutbound.Port,
		))
		xc.Reverse.Bridges = append(xc.Reverse.Bridges, &xray.ReverseItem{
			Tag:    "bridge",
			Domain: fmt.Sprintf("s%d.google.com", s.Id),
		})
		xc.Routing.Settings.Rules = append(
			xc.Routing.Settings.Rules,
			&xray.Rule{
				Type:        "field",
				InboundTag:  []string{"bridge"},
				Domain:      []string{fmt.Sprintf("full:s%d.google.com", s.Id)},
				OutboundTag: "foreign",
			},
			&xray.Rule{
				Type:        "field",
				InboundTag:  []string{"bridge"},
				OutboundTag: "freedom",
			},
		)
	}

	return xc
}

func New(logger *logger.Logger, database *database.Database, xray *xray.Xray) *Writer {
	return &Writer{l: logger, database: database, xray: xray}
}