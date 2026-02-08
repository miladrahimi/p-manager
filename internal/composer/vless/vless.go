package composer

import xraycomponent "github.com/miladrahimi/p-node/pkg/xray/config/component"

const (
	vlessRealityFlow     = "xtls-rprx-vision"
	vlessRealityNetwork  = "tcp"
	vlessRealitySecurity = "reality"
)

// NewVlessRealityVisionInbound builds a VLESS+Reality inbound with vision flow.
func NewVlessRealityVisionInbound(
	port int,
	clientID string,
	dest string,
	serverNames []string,
	privateKey string,
	shortIDs []string,
) *xraycomponent.Inbound {
	return &xraycomponent.Inbound{
		Port:     port,
		Protocol: "vless",
		Settings: &xraycomponent.InboundSettings{
			Clients: []*xraycomponent.VlessUser{
				{
					Id:   clientID,
					Flow: vlessRealityFlow,
				},
			},
			Decryption: "none",
		},
		StreamSettings: &xraycomponent.StreamSettings{
			Network:  vlessRealityNetwork,
			Security: vlessRealitySecurity,
			RealitySettings: &xraycomponent.RealitySettings{
				Dest:        dest,
				ServerNames: serverNames,
				PrivateKey:  privateKey,
				ShortIds:    shortIDs,
			},
		},
		Sniffing: &xraycomponent.Sniffing{
			Enabled:      true,
			DestOverride: []string{"http", "tls", "quic"},
			RouteOnly:    true,
		},
	}
}
