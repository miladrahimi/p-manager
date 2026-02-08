package data

// XraySettings represents the configuration for XraySettings.
type XraySettings struct {
	VtrDirectPort int    `json:"vtr_direct_port" validate:"min=0,max=65535"`
	VtrRemotePort int    `json:"vtr_remote_port" validate:"min=0,max=65535"`
	Vt2VtrPort    int    `json:"vt_2_vtr_port" validate:"min=0,max=65535"`
	VtrPrivateKey string `json:"vtr_private_key"`
	VtrPublicKey  string `json:"vtr_public_key"`
}

// NewXraySettings creates a new instance of XraySettings.
func NewXraySettings(vlessPrivateKey string, vlessPublicKey string) *XraySettings {
	return &XraySettings{
		VtrPrivateKey: vlessPrivateKey,
		VtrPublicKey:  vlessPublicKey,
	}
}

func DefaultXray() *XraySettings {
	return NewXraySettings("", "")
}
