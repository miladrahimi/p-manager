package data

import "github.com/miladrahimi/p-manager/internal/config"

// XraySettings represents the configuration for XraySettings.
type XraySettings struct {
	RrDirectPort int    `json:"rr_direct_port" validate:"min=0,max=65535"`
	RrRemotePort int    `json:"rr_remote_port" validate:"min=0,max=65535"`
	Rr2RrPort    int    `json:"rr_2_rr_port" validate:"min=0,max=65535"`
	Rr2SshPort   int    `json:"rr_2_ssh_port" validate:"min=0,max=65535"`
	RrPrivateKey string `json:"rr_private_key"`
	RrPublicKey  string `json:"rr_public_key"`
	NodeSni      string `json:"node_sni" validate:"required,hostname_rfc1123"`
	ManagerSni   string `json:"manager_sni" validate:"required,hostname_rfc1123"`
}

// NewXraySettings creates a new instance of XraySettings.
func NewXraySettings(vlessPrivateKey string, vlessPublicKey string, nodeSni string, managerSni string) *XraySettings {
	return &XraySettings{
		RrPrivateKey: vlessPrivateKey,
		RrPublicKey:  vlessPublicKey,
		NodeSni:      nodeSni,
		ManagerSni:   managerSni,
	}
}

func DefaultXray() *XraySettings {
	return NewXraySettings("", "", config.DefaultNodeSni, config.DefaultManagerSni)
}
