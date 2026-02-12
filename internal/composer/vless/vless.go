package vless

import (
	"net"

	"github.com/miladrahimi/p-node/pkg/xray/config/component"
)

const (
	Protocol        = "vless"
	FlowVision      = "xtls-rprx-vision"
	NetworkRaw      = "raw"
	SecurityReality = "reality"
	EncryptionNone  = "none"
	EncryptionEmpty = ""
)

// MakeUser makes a VLESS user.
func MakeUser(id, flow, encryption string) *component.VlessUser {
	return &component.VlessUser{
		Id:         id,
		Flow:       flow,
		Encryption: encryption,
	}
}

// MakeVrrvInbound makes a VLESS/Raw/Reality/Vision inbound.
func MakeVrrvInbound(
	tag string,
	port int,
	privateKey string,
	sni string,
	clients []*component.VlessUser,
	fallback *component.VlessFallback,
) *component.Inbound {
	return &component.Inbound{
		Tag:      tag,
		Port:     port,
		Protocol: Protocol,
		Settings: &component.InboundSettings{
			Clients:    clients,
			Decryption: EncryptionNone,
			Fallbacks:  []*component.VlessFallback{fallback},
		},
		StreamSettings: &component.StreamSettings{
			Network:  NetworkRaw,
			Security: SecurityReality,
			RealitySettings: &component.RealitySettings{
				Dest:        net.JoinHostPort(sni, "443"),
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
}

// MakeVrInbound makes a VLESS/Raw inbound without reality and flow.
func MakeVrInbound(
	tag string,
	port int,
	clients []*component.VlessUser,
	fallback *component.VlessFallback,
) *component.Inbound {
	return &component.Inbound{
		Tag:      tag,
		Port:     port,
		Protocol: Protocol,
		Settings: &component.InboundSettings{
			Clients:    clients,
			Decryption: EncryptionNone,
			Fallbacks:  []*component.VlessFallback{fallback},
		},
		StreamSettings: &component.StreamSettings{
			Network: NetworkRaw,
		},
	}
}

// MakeVrrvOutbound makes a VLESS/Raw/Reality outbound.
func MakeVrrvOutbound(
	tag string,
	address string,
	port int,
	id string,
	publicKey string,
	nodeSni string,
) *component.Outbound {
	user := MakeUser(id, FlowVision, EncryptionNone)

	return &component.Outbound{
		Tag:      tag,
		Protocol: Protocol,
		Settings: &component.OutboundSettings{
			Vnext: []*component.VlessOutboundServer{
				{
					Address: address,
					Port:    port,
					Users:   []*component.VlessUser{user},
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
