package composer

import (
	"fmt"
	"time"

	"github.com/miladrahimi/p-manager/internal/composer/socks"
	"github.com/miladrahimi/p-manager/internal/composer/vless"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-manager/pkg/util"
	xrayConfig "github.com/miladrahimi/p-node/pkg/xray/config"
	"github.com/miladrahimi/p-node/pkg/xray/config/component"
)

// ManagerConfig composes the P-Manager (local) xray config. The api inbound
// port is left at its default; Xray.Reconfigure replaces it with the port of
// the running instance.
func (c *Composer) ManagerConfig(
	sshConfigsByNodeIds map[string][]*ssh.ProxyConfig,
	paqetPortsByNodeIds map[string]int,
) *xrayConfig.Config {
	var xc *xrayConfig.Config
	c.db.Read(func(d *data.Data) {
		xc = c.composeManagerConfig(d, sshConfigsByNodeIds, paqetPortsByNodeIds)
	})
	return xc
}

// composeManagerConfig builds the manager config. The caller must hold the
// store read lock.
func (c *Composer) composeManagerConfig(
	d *data.Data,
	sshConfigsByNodeIds map[string][]*ssh.ProxyConfig,
	paqetPortsByNodeIds map[string]int,
) *xrayConfig.Config {
	rrClients := c.rrClients(d)
	xs := d.XraySettings
	xc := xrayConfig.New(c.config.Xray.LogLevel)

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
		balancer := &component.Balancer{Tag: "relay-rr2rr", Selector: []string{}}
		xc.Routing.Balancers = append(xc.Routing.Balancers, balancer)
		for _, n := range d.Nodes {
			xc.Outbounds = append(xc.Outbounds, vless.MakeRrOutbound(
				fmt.Sprintf("relay-rr2rr-%s", n.Id),
				n.Host,
				xs.RelayRr2RrNodePort,
				RelayRr2RrClientId(n.Id, xs.RealityPrivateKey),
				xs.RealityPublicKey,
				xs.NodeSni,
				vless.FlowVision,
			))
			balancer.Selector = append(balancer.Selector, fmt.Sprintf("relay-rr2rr-%s", n.Id))
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

	// Relay RR2PQ: accounts are balanced over the local SOCKS proxies of the
	// paqet clients, one per node (started by the coordinator).
	if hasClients && hasNodes && xs.RelayRr2PqPort > 0 && xs.RelayRr2PqNodePort > 0 {
		outbounds := make([]string, 0, len(d.Nodes))
		for _, n := range d.Nodes {
			socksPort, ok := paqetPortsByNodeIds[n.Id]
			if !ok || socksPort <= 0 {
				continue
			}
			tag := fmt.Sprintf("relay-rr2pq-%s", n.Id)
			xc.Outbounds = append(xc.Outbounds, socks.MakeOutbound(tag, "127.0.0.1", socksPort))
			outbounds = append(outbounds, tag)
		}

		if len(outbounds) > 0 {
			xc.Inbounds = append(xc.Inbounds, vless.MakeRrInbound(
				"relay-rr2pq",
				xs.RelayRr2PqPort,
				xs.RealityPrivateKey,
				xs.ManagerSni,
				rrClients,
				fallback,
			))
			xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
				InboundTag:  []string{"relay-rr2pq"},
				BalancerTag: "relay-rr2pq",
			})
			xc.Routing.Balancers = append(xc.Routing.Balancers, &component.Balancer{
				Tag:      "relay-rr2pq",
				Selector: outbounds,
			})
		}
	}

	// Reverse RR: P-Manager is the portal. Accounts connect to the manager
	// port, every P-Node bridge dials the portal port, and the portal
	// load-balances accounts across the connected bridges.
	if hasClients && hasNodes && xs.ReverseRrManagerPort > 0 && xs.ReverseRrPortalPort > 0 {
		bridges := make([]*component.Client, 0, len(d.Nodes))
		for _, n := range d.Nodes {
			// No flow: the bridge tunnel carries mux, incompatible with vision.
			bridges = append(bridges, vless.MakeUser(
				ReverseRrBridgeId(n.Id, xs.RealityPrivateKey),
				vless.FlowNone,
				vless.EncryptionEmpty,
			))
		}

		xc.Inbounds = append(xc.Inbounds,
			vless.MakeRrInbound(
				"reverse-rr",
				xs.ReverseRrManagerPort,
				xs.RealityPrivateKey,
				xs.ManagerSni,
				rrClients,
				fallback,
			),
			vless.MakeRrInbound(
				"reverse-rr-tunnel",
				xs.ReverseRrPortalPort,
				xs.RealityPrivateKey,
				xs.ManagerSni,
				bridges,
				fallback,
			),
		)
		xc.Reverse.Portals = append(xc.Reverse.Portals, &component.ReverseItem{
			Tag:    "reverse-rr-portal",
			Domain: reverseRrDomain,
		})
		// The portal sorts bridge registrations from account traffic by target.
		xc.Routing.Rules = append(xc.Routing.Rules, &component.Rule{
			InboundTag:  []string{"reverse-rr", "reverse-rr-tunnel"},
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

	// No HTTP port (push disabled) means no fallback; dest 0 is invalid.
	var fallback *component.Fallback
	if node.HttpPort > 0 {
		fallback = &component.Fallback{Dest: node.HttpPort}
	}

	xc.Metadata = &component.Metadata{
		UpdatedAt: lastUpdate.Format(time.RFC3339),
		UpdatedBy: d.MainSettings.Host,
	}

	// The relay client id is derived deterministically, so it matches the
	// manager's relay outbound without reading the manager's live config.
	relayOn := len(c.rrClients(d)) > 0 && xs.RelayRr2RrManagerPort > 0 && xs.RelayRr2RrNodePort > 0
	relayClient := vless.MakeUser(RelayRr2RrClientId(node.Id, xs.RealityPrivateKey), vless.FlowVision, vless.EncryptionEmpty)
	// With a shared port the remote-rr inbound below serves the relay too.
	shared := relayOn && xs.NodeRrShared()
	if relayOn && !shared {
		clients := []*component.Client{relayClient}
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
	if len(c.rrClients(d)) > 0 && xs.ReverseRrManagerPort > 0 && xs.ReverseRrPortalPort > 0 && d.MainSettings.Host != "" {
		bridgeId := ReverseRrBridgeId(node.Id, xs.RealityPrivateKey)
		// No flow: the reverse tunnel carries mux, incompatible with vision.
		xc.Outbounds = append(xc.Outbounds, vless.MakeRrOutbound(
			"reverse-rr-tunnel",
			d.MainSettings.Host,
			xs.ReverseRrPortalPort,
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

	// Relay RR2PQ: the node runs a paqet server the manager's paqet client dials.
	if len(c.rrClients(d)) > 0 && xs.RelayRr2PqPort > 0 && xs.RelayRr2PqNodePort > 0 {
		xc.Paqet = &component.Paqet{
			Port: xs.RelayRr2PqNodePort,
			Key:  RelayRr2PqKey(node.Id, xs.RealityPrivateKey),
		}
	}

	if xs.RemoteRrPort > 0 {
		clients := c.rrClients(d)
		if shared {
			clients = append(clients, relayClient)
		}
		xc.Inbounds = append(xc.Inbounds, vless.MakeRrInbound(
			"remote-rr",
			xs.RemoteRrPort,
			xs.RealityPrivateKey,
			xs.NodeSni,
			clients,
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

	// Remote CXH: clients reach the node through Cloudflare, which connects
	// to this port with the Origin CA certificate the manager issued.
	if xs.RemoteCxhPort > 0 && d.Cdn.Ready() {
		xc.Inbounds = append(xc.Inbounds, vless.MakeCxhInbound(
			"remote-cxh",
			xs.RemoteCxhPort,
			d.Cdn.CxhPath,
			d.Cdn.Certificate,
			d.Cdn.PrivateKey,
			c.cxhClients(d),
		))
		xc.Routing.Rules = append(
			xc.Routing.Rules,
			&component.Rule{
				InboundTag:  []string{"remote-cxh"},
				OutboundTag: "out",
			},
		)
	}

	return xc
}

// reverseRrDomain pairs the reverse portal with the bridges; all nodes share it
// so the portal pools and load-balances their tunnels.
const reverseRrDomain = "reverse-rr"

// RelayRr2RrClientId is the stable relay client id shared by the manager
// outbound and the node inbound (each derives it independently).
func RelayRr2RrClientId(nodeId, realityPrivateKey string) string {
	return util.StableUuid("relay-rr2rr|" + nodeId + "|" + realityPrivateKey)
}

// ReverseRrBridgeId is the stable reverse-tunnel client id shared by the manager
// inbound and the node outbound (each derives it independently).
func ReverseRrBridgeId(nodeId, realityPrivateKey string) string {
	return util.StableUuid("reverse-rr|" + nodeId + "|" + realityPrivateKey)
}

// RelayRr2PqKey is the stable paqet shared key for a node, derived on both
// sides from the node id and the Reality private key.
func RelayRr2PqKey(nodeId, realityPrivateKey string) string {
	return util.StableUuid("relay-rr2pq|" + nodeId + "|" + realityPrivateKey)
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

// cxhClients returns the XHTTP client list (no flow). The caller must hold the
// store read lock.
func (c *Composer) cxhClients(d *data.Data) []*component.Client {
	var clients []*component.Client
	for _, u := range d.Accounts {
		if !u.Enabled {
			continue
		}
		clients = append(clients, vless.MakeUser(u.ProxyId, vless.FlowNone, vless.EncryptionEmpty))
	}
	return clients
}

// LogLabels maps every identifier that can show up in log lines (account ids
// and proxy ids, node ids and the client/bridge ids derived from them) to a
// human-readable label, so the log viewer can annotate them.
func (c *Composer) LogLabels() map[string]string {
	labels := map[string]string{}
	c.db.Read(func(d *data.Data) {
		for _, a := range d.Accounts {
			labels[a.Id] = a.Name
			if a.ProxyId != "" {
				labels[a.ProxyId] = a.Name
			}
		}
		for _, n := range d.Nodes {
			labels[n.Id] = n.Host
			// The relay client shows up in node logs as the manager's connection.
			labels[RelayRr2RrClientId(n.Id, d.XraySettings.RealityPrivateKey)] = "relay from P-Manager"
			labels[ReverseRrBridgeId(n.Id, d.XraySettings.RealityPrivateKey)] = n.Host
		}
	})
	return labels
}
