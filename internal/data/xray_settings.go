package data

// XraySettings represents the configuration for XraySettings.
type XraySettings struct {
	VrrvDirectPort int    `json:"vrrv_direct_port" validate:"min=0,max=65535"`
	VrrvRemotePort int    `json:"vrrv_remote_port" validate:"min=0,max=65535"`
	Vrrv2VrrvPort   int    `json:"vrrv_2_vrrv_port" validate:"min=0,max=65535"`
	VrrvPrivateKey string `json:"vrrv_private_key"`
	VrrvPublicKey  string `json:"vrrv_public_key"`
}

// NewXraySettings creates a new instance of XraySettings.
func NewXraySettings(vlessPrivateKey string, vlessPublicKey string) *XraySettings {
	return &XraySettings{
		VrrvPrivateKey: vlessPrivateKey,
		VrrvPublicKey:  vlessPublicKey,
	}
}

func DefaultXray() *XraySettings {
	return NewXraySettings("", "")
}
