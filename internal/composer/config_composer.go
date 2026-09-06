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
	apiPort, err := util.FreePort()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var xc *xrayConfig.Config
	c.db.Read(func(d *data.Data) {
		xc = c.composeManagerConfig(d, sshConfigsByNodeIds, apiPort)
	})
	return xc, nil
}

// composeManagerConfig builds the manager config. The caller must hold the
// store read lock.
func (c *Composer) composeManagerConfig(
	d *data.Data,
	sshConfigsByNodeIds map[string][]*ssh.ProxyConfig,
	apiPort int,
) *xrayConfig.Config {
	rrClients := c.rrClients(d)
	xs := d.XraySettings
	xc := xrayConfig.New(c.config.Xray.LogLevel)

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
				rr2rrClientId(n.Id, xs.RealityPrivateKey),
				xs.RealityPublicKey,
				xs.NodeSni,
				vless.FlowVision,
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

	// Reverse RR: P-Manager is the portal; accounts and every P-Node bridge
	// connect to this inbound, and the portal load-balances accounts across the
	// connected bridges.
	if hasClients && hasNodes && xs.ReverseRrManagerPort > 0 {
		clients := make([]*component.Client, 0, len(rrClients)+len(d.Nodes))
		clients = append(clients, rrClients...)
		for _, n := range d.Nodes {
			// No flow: the bridge tunnel carries mux, incompatible with vision.
			clients = append(clients, vless.MakeUser(
				reverseRrBridgeId(n.Id, xs.RealityPrivateKey),
				vless.FlowNone,
				vless.EncryptionEmpty,
			))
		}

		xc.Inbounds = append(xc.Inbounds, vless.MakeRrInbound(
			"reverse-rr",
			xs.ReverseRrManagerPort,
			xs.RealityPrivateKey,
			xs.ManagerSni,
			clients,
			fallback,
		))
		xc.Reverse.Portals = append(xc.Reverse.Portals, &component.ReverseItem{
			Tag:    "reverse-rr-portal",
			Domain: reverseRrDomain,
		})
		// The portal sorts bridge registrations from account traffic by target.
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"reverse-rr"},
			OutboundTag: "reverse-rr-portal",
		})
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

	return xc
}

// NodeConfig composes the (remote) P-Node xray config.
func (c *Composer) NodeConfig(node *data.Node, lastUpdate time.Time) *xrayConfig.Config {
	var xc *xrayConfig.Config
	c.db.Read(func(d *data.Data) {
		xc = c.composeNodeConfig(d, node, lastUpdate)
	})
	return xc
}

// composeNodeConfig builds the node config. The caller must hold the store
// read lock.
func (c *Composer) composeNodeConfig(d *data.Data, node *data.Node, lastUpdate time.Time) *xrayConfig.Config {
	xs := d.XraySettings
	xc := xrayConfig.New(c.config.Xray.LogLevel)
	fallback := &component.Fallback{Dest: node.HttpPort}

	xc.Metadata = &component.Metadata{
		UpdatedAt: lastUpdate.Format(time.RFC3339),
		UpdatedBy: d.MainSettings.Host,
	}

	// The relay client id is derived deterministically, so it matches the
	// manager's relay outbound without reading the manager's live config.
	if len(c.rrClients(d)) > 0 && xs.RelayRr2RrManagerPort > 0 && xs.RelayRr2RrNodePort > 0 {
		clientId := rr2rrClientId(node.Id, xs.RealityPrivateKey)
		clients := []*component.Client{vless.MakeUser(clientId, vless.FlowVision, vless.EncryptionEmpty)}
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

	// Reverse RR: P-Node is the bridge; it dials out to the manager portal, so
	// it needs only the manager host (delivered with this config), not its own.
	if len(c.rrClients(d)) > 0 && xs.ReverseRrManagerPort > 0 && d.MainSettings.Host != "" {
		bridgeId := reverseRrBridgeId(node.Id, xs.RealityPrivateKey)
		// No flow: the reverse tunnel carries mux, incompatible with vision.
		xc.Outbounds = append(xc.Outbounds, vless.MakeRrOutbound(
			"reverse-rr-tunnel",
			d.MainSettings.Host,
			xs.ReverseRrManagerPort,
			bridgeId,
			xs.RealityPublicKey,
			xs.ManagerSni,
			vless.FlowNone,
		))
		xc.Reverse.Bridges = append(xc.Reverse.Bridges, &component.ReverseItem{
			Tag:    "reverse-rr-bridge",
			Domain: reverseRrDomain,
		})
		xc.Routing.Rules = append(
			xc.Routing.Rules,
			// Tunnel connections (target == reverse domain) dial the portal.
			&component.Rule{
				InboundTag:  []string{"reverse-rr-bridge"},
				Domain:      []string{"full:" + reverseRrDomain},
				OutboundTag: "reverse-rr-tunnel",
			},
			// Account traffic returning through the tunnel goes to the internet.
			&component.Rule{
				InboundTag:  []string{"reverse-rr-bridge"},
				OutboundTag: "out",
			},
		)
	}

	if xs.RemoteRrPort > 0 {
		xc.Inbounds = append(xc.Inbounds, vless.MakeRrInbound(
			"remote-rr",
			xs.RemoteRrPort,
			xs.RealityPrivateKey,
			xs.NodeSni,
			c.rrClients(d),
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

// reverseRrDomain pairs the reverse portal with the bridges; all nodes share it
// so the portal pools and load-balances their tunnels.
const reverseRrDomain = "reverse-rr"

// rr2rrClientId is the stable relay client id shared by the manager outbound and
// the node inbound (each derives it independently).
func rr2rrClientId(nodeId, realityPrivateKey string) string {
	return util.StableUuid("relay-rr2rr|" + nodeId + "|" + realityPrivateKey)
}

// reverseRrBridgeId is the stable reverse-tunnel client id shared by the manager
// inbound and the node outbound (each derives it independently).
func reverseRrBridgeId(nodeId, realityPrivateKey string) string {
	return util.StableUuid("reverse-rr|" + nodeId + "|" + realityPrivateKey)
}

// rrClients returns the RR-ready client list. The caller must hold the store
// read lock.
func (c *Composer) rrClients(d *data.Data) []*component.Client {
	var clients []*component.Client
	for _, u := range d.Accounts {
		if !u.Enabled {
			continue
		}
		clients = append(clients, vless.MakeUser(u.ProxyId, vless.FlowVision, vless.EncryptionEmpty))
	}
	return clients
}
