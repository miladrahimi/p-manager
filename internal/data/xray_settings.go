package data

import "github.com/miladrahimi/p-manager/internal/config"

// XraySettings represents the configuration for XraySettings.
type XraySettings struct {
	VrrvDirectPort int    `json:"vrrv_direct_port" validate:"min=0,max=65535"`
	VrrvRemotePort int    `json:"vrrv_remote_port" validate:"min=0,max=65535"`
	Vrrv2VrrvPort  int    `json:"vrrv_2_vrrv_port" validate:"min=0,max=65535"`
	Vrrv2SshPort   int    `json:"vrrv_2_ssh_port" validate:"min=0,max=65535"`
	VrrvPrivateKey string `json:"vrrv_private_key"`
	VrrvPublicKey  string `json:"vrrv_public_key"`
	NodeSni        string `json:"node_sni" validate:"required,hostname_rfc1123"`
	ManagerSni     string `json:"manager_sni" validate:"required,hostname_rfc1123"`
}

// NewXraySettings creates a new instance of XraySettings.
func NewXraySettings(vlessPrivateKey string, vlessPublicKey string, nodeSni string, managerSni string) *XraySettings {
	return &XraySettings{
		VrrvPrivateKey: vlessPrivateKey,
		VrrvPublicKey:  vlessPublicKey,
		NodeSni:        nodeSni,
		ManagerSni:     managerSni,
	}
}

func DefaultXray() *XraySettings {
	return NewXraySettings("", "", config.DefaultNodeSni, config.DefaultManagerSni)
}
