package composer

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/http/client"
	"github.com/miladrahimi/p-node/pkg/xray"
	xrayConfig "github.com/miladrahimi/p-node/pkg/xray/config"
	"github.com/miladrahimi/p-node/pkg/xray/config/component"
)

// Composer composes the xray config.
type Composer struct {
	config *config.Config
	hc     *client.Client
	db     *database.Database[data.Data]
	xray   *xray.Xray
}

// New creates a new composer.
func New(config *config.Config, database *database.Database[data.Data], xray *xray.Xray) *Composer {
	return &Composer{config: config, db: database, xray: xray}
}

// LocalConfig composes the local xray config.
func (w *Composer) LocalConfig() (*xrayConfig.Config, error) {
	clients := w.clients()
	d := w.db.Data()

	apiPort, err := util.FreePort()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	xc := xrayConfig.New(w.config.Xray.LogLevel)

	xc.FindInbound("api").Port = apiPort

	if len(clients) > 0 {
		if d.XraySettings.Vrrv2VrrvPort > 0 {
			xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
				"relay",
				d.XraySettings.Vrrv2VrrvPort,
				d.XraySettings.VrrvPrivateKey,
				clients,
			))
		}

		if d.XraySettings.VrrvDirectPort > 0 {
			xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
				"direct",
				d.XraySettings.VrrvDirectPort,
				d.XraySettings.VrrvPrivateKey,
				clients,
			))
		}
	}

	if len(clients) > 0 {
		if d.XraySettings.VrrvDirectPort > 0 {
			xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
				InboundTag:  []string{"direct"},
				OutboundTag: "out",
			})
		}
		if len(d.Nodes) > 0 {
			if d.XraySettings.Vrrv2VrrvPort > 0 {
				xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
					InboundTag:  []string{"relay"},
					BalancerTag: "relay",
				})
			}
		}
	}

	if len(d.Nodes) > 0 {
		if d.XraySettings.Vrrv2VrrvPort > 0 {
			xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
				Tag: "relay", Selector: []string{},
			})
		}
	}

	for _, n := range d.Nodes {
		if d.XraySettings.Vrrv2VrrvPort > 0 {
			outboundRelayPort, err := util.FreePort()
			if err != nil {
				return nil, errors.WithStack(err)
			}
			xc.Outbounds = append(xc.Outbounds, vless.MakeVrrvOutbound(
				fmt.Sprintf("relay-%s", n.Id),
				n.Host,
				outboundRelayPort,
				util.Uuid(),
				d.XraySettings.VrrvPublicKey,
			))
			xc.FindBalancer("relay").Selector = append(
				xc.FindBalancer("relay").Selector,
				fmt.Sprintf("relay-%s", n.Id),
			)
		}
	}

	return xc, nil
}

// NodeConfig composes the (remote) node xray config.
func (w *Composer) NodeConfig(node *data.Node, lastUpdate time.Time, password string) *xrayConfig.Config {
	d := w.db.Data()
	xc := xrayConfig.New(w.config.Xray.LogLevel)

	xc.Metadata = &component.Metadata{
		UpdatedAt: lastUpdate.Format(time.RFC3339),
		UpdatedBy: d.MainSettings.Host,
	}

	if d.XraySettings.Vrrv2VrrvPort > 0 {
		relayOutbound := w.xray.Config().FindOutbound(fmt.Sprintf("relay-%s", node.Id))
		if relayOutbound != nil && relayOutbound.Settings != nil {
			if len(relayOutbound.Settings.Vnext) > 0 {
				server := relayOutbound.Settings.Vnext[0]
				users := make([]*component.VlessUser, len(server.Users))
				for i, u := range server.Users {
					users[i] = vless.MakeUser(u.Id, vless.FlowVision, vless.EncryptionEmpty)
				}
				xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
					"direct",
					server.Port,
					d.XraySettings.VrrvPrivateKey,
					users,
				))
				xc.Routing.Rules = append(
					xc.Routing.Rules,
					&component.Rule{
						InboundTag:  []string{"direct"},
						OutboundTag: "out",
					},
				)
			}
		}
	}

	if w.db.Data().XraySettings.VrrvRemotePort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
			"remote",
			d.XraySettings.VrrvRemotePort,
			d.XraySettings.VrrvPrivateKey,
			w.clients(),
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
func (w *Composer) clients() []*component.VlessUser {
	var clients []*component.VlessUser
	for _, u := range w.db.Data().Users {
		if !u.Enabled {
			continue
		}
		clients = append(clients, vless.MakeUser(u.VlessId, vless.FlowVision, vless.EncryptionEmpty))
	}
	return clients
}
