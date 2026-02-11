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
func (w *Composer) LocalConfig() (*xrayConfig.Config, error) {
	clients := w.clients()
	d := w.db.Data()
	xs := d.XraySettings
	xc := xrayConfig.New(w.config.Xray.LogLevel)

	apiPort, err := util.FreePort()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	xc.FindInbound("api").Port = apiPort

	hasClients := len(clients) > 0
	hasNodes := len(d.Nodes) > 0

	if hasClients && hasNodes && xs.Vrrv2VrrvPort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
			"relay",
			xs.Vrrv2VrrvPort,
			xs.VrrvPrivateKey,
			xs.ManagerSni,
			clients,
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

	if hasClients && xs.VrrvDirectPort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeVrrvInbound(
			"direct",
			xs.VrrvDirectPort,
			xs.VrrvPrivateKey,
			xs.ManagerSni,
			clients,
		))
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"direct"},
			OutboundTag: "out",
		})
	}

	return xc, nil
}

// NodeConfig composes the (remote) node xray config.
func (w *Composer) NodeConfig(node *data.Node, lastUpdate time.Time, password string) *xrayConfig.Config {
	d := w.db.Data()
	xs := d.XraySettings
	xc := xrayConfig.New(w.config.Xray.LogLevel)

	xc.Metadata = &component.Metadata{
		UpdatedAt: lastUpdate.Format(time.RFC3339),
		UpdatedBy: d.MainSettings.Host,
	}

	if xs.Vrrv2VrrvPort > 0 {
		relayOutbound := w.xray.Config().FindOutbound(fmt.Sprintf("relay-%s", node.Id))
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
