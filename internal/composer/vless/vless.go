package vless

import (
	"net"

	"github.com/miladrahimi/p-node/pkg/xray/config/component"
)

const (
	Protocol        = "vless"
	FlowVision      = "xtls-rprx-vision"
	FlowNone        = ""
	NetworkRaw      = "raw"
	SecurityReality = "reality"
	EncryptionNone  = "none"
	EncryptionEmpty = ""
)

// MakeUser makes a VLESS user.
func MakeUser(id, flow, encryption string) *component.Client {
	return &component.Client{
		Id:         id,
		Flow:       flow,
		Email:      id,
		Encryption: encryption,
	}
}

// MakeRrInbound makes a Reality Raw inbound.
func MakeRrInbound(
	tag string,
	port int,
	privateKey string,
	sni string,
	clients []*component.Client,
	fallback *component.Fallback,
) *component.Inbound {
	inbound := &component.Inbound{
		Tag:      tag,
		Port:     port,
		Protocol: Protocol,
		Settings: &component.InboundSettings{
			Clients:    clients,
			Decryption: EncryptionNone,
			Fallbacks:  []*component.Fallback{fallback},
		},
		StreamSettings: &component.StreamSettings{
			Network:  NetworkRaw,
			Security: SecurityReality,
			RealitySettings: &component.RealitySettings{
				Dest:        net.JoinHostPort(sni, "443"),
				Target:      net.JoinHostPort(sni, "443"),
				PrivateKey:  privateKey,
				ServerNames: []string{sni},
				ShortIds:    []string{""},
			},
		},
		Sniffing: &component.Sniffing{
			Enabled:      true,
			RouteOnly:    true,
			DestOverride: []string{"http", "tls", "quic"},
		},
	}

	return inbound
}

// MakeRrOutbound makes a Reality Raw outbound. The flow is usually FlowVision;
// pass EncryptionEmpty (no flow) for tunnels that carry mux (e.g. the reverse
// bridge), since xtls-rprx-vision is incompatible with mux.
func MakeRrOutbound(
	tag string,
	address string,
	port int,
	id string,
	publicKey string,
	nodeSni string,
	flow string,
) *component.Outbound {
	user := MakeUser(id, flow, EncryptionNone)

	return &component.Outbound{
		Tag:      tag,
		Protocol: Protocol,
		Settings: &component.OutboundSettings{
			Vnext: []*component.Vnext{
				{
					Address: address,
					Port:    port,
					Users:   []*component.Client{user},
				},
			},
		},
		StreamSettings: &component.StreamSettings{
			Network:  NetworkRaw,
			Security: SecurityReality,
			RealitySettings: &component.RealitySettings{
				Fingerprint: "chrome",
				ServerName:  nodeSni,
				PublicKey:   publicKey,
				ShortId:     "",
			},
		},
	}
}
