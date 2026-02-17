package data

import "github.com/miladrahimi/p-manager/internal/config"

// XraySettings represents the configuration for XraySettings.
type XraySettings struct {
	DirectRrPort      int    `json:"direct_rr_port" validate:"min=0,max=65535"`
	RemoteRrPort      int    `json:"remote_rr_port" validate:"min=0,max=65535"`
	RelayRr2RrPort    int    `json:"relay_rr_2_rr_port" validate:"min=0,max=65535"`
	RelayRr2SshPort   int    `json:"relay_rr_2_ssh_port" validate:"min=0,max=65535"`
	RealityPrivateKey string `json:"reality_private_key"`
	RealityPublicKey  string `json:"reality_public_key"`
	NodeSni           string `json:"node_sni" validate:"required,hostname_rfc1123"`
	ManagerSni        string `json:"manager_sni" validate:"required,hostname_rfc1123"`
}

// NewXraySettings creates a new instance of XraySettings.
func NewXraySettings(vlessPrivateKey string, vlessPublicKey string, nodeSni string, managerSni string) *XraySettings {
	return &XraySettings{
		RealityPrivateKey: vlessPrivateKey,
		RealityPublicKey:  vlessPublicKey,
		NodeSni:           nodeSni,
		ManagerSni:        managerSni,
	}
}

func DefaultXray() *XraySettings {
	return NewXraySettings("", "", config.DefaultNodeSni, config.DefaultManagerSni)
}
