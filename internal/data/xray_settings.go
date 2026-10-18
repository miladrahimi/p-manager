package data

import "github.com/miladrahimi/p-manager/internal/config"

// XraySettings represents the configuration for XraySettings.
type XraySettings struct {
	DirectRrPort           int    `json:"direct_rr_port" validate:"min=0,max=65535"`
	RemoteRrPort           int    `json:"remote_rr_port" validate:"min=0,max=65535"`
	RelayRr2RrManagerPort  int    `json:"relay_rr_2_rr_manager_port" validate:"min=0,max=65535"`
	RelayRr2RrNodePort     int    `json:"relay_rr_2_rr_node_port" validate:"min=0,max=65535"`
	ReverseRrManagerPort   int    `json:"reverse_rr_manager_port" validate:"min=0,max=65535"`
	ReverseRrPortalPort    int    `json:"reverse_rr_portal_port" validate:"min=0,max=65535"`
	RelayRr2SshPort        int    `json:"relay_rr_2_ssh_port" validate:"min=0,max=65535"`
	RelayRr2SshConnections int    `json:"relay_rr_2_ssh_connections" validate:"min=1,max=32"`
	RelayRr2PqPort         int    `json:"relay_rr_2_pq_port" validate:"min=0,max=65535"`
	RelayRr2PqNodePort     int    `json:"relay_rr_2_pq_node_port" validate:"min=0,max=65535"`
	RealityPrivateKey      string `json:"reality_private_key"`
	RealityPublicKey       string `json:"reality_public_key"`
	NodeSni                string `json:"node_sni" validate:"required,hostname_rfc1123"`
	ManagerSni             string `json:"manager_sni" validate:"required,hostname_rfc1123"`
}

// NewXraySettings creates a new instance of XraySettings.
func NewXraySettings(vlessPrivateKey string, vlessPublicKey string, nodeSni string, managerSni string) *XraySettings {
	return &XraySettings{
		RealityPrivateKey:      vlessPrivateKey,
		RealityPublicKey:       vlessPublicKey,
		NodeSni:                nodeSni,
		ManagerSni:             managerSni,
		RelayRr2SshConnections: 8,
	}
}

// NodeRrShared reports whether Remote RR and Relay RR2RR use the same node
// port. The node then serves both from one inbound, so a single port (e.g.
// 443) can carry account clients and the manager's relay at the same time.
func (x *XraySettings) NodeRrShared() bool {
	return x.RemoteRrPort > 0 && x.RelayRr2RrManagerPort > 0 && x.RelayRr2RrNodePort == x.RemoteRrPort
}

func DefaultXray() *XraySettings {
	return NewXraySettings("", "", config.DefaultNodeSni, config.DefaultManagerSni)
}
