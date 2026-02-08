package data

// Xray represents the configuration for Xray.
type Xray struct {
	VtrDirectPort int    `json:"vtr_direct_port" validate:"min=0,max=65535"`
	VtrRemotePort int    `json:"vtr_remote_port" validate:"min=0,max=65535"`
	Vt2VtrPort    int    `json:"vt_2_vtr_port" validate:"min=0,max=65535"`
	VtrPrivateKey string `json:"vtr_private_key"`
	VtrPublicKey  string `json:"vtr_public_key"`
}

// NewXray creates a new instance of Xray.
func NewXray(vlessPrivateKey string, vlessPublicKey string) *Xray {
	return &Xray{
		VtrPrivateKey: vlessPrivateKey,
		VtrPublicKey:  vlessPublicKey,
	}
}

func DefaultXray() *Xray {
	return NewXray("", "")
}
