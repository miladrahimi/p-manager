package vless

import (
	"github.com/miladrahimi/p-node/pkg/xray/config/component"
)

const (
	Protocol                = "vless"
	FlowVision              = "xtls-rprx-vision"
	NetworkTcp              = "tcp"
	SecurityReality         = "reality"
	ServerNameStackOverflow = "stackoverflow.com"
	DestStackOverflow       = "stackoverflow.com:443"
	EncryptionNone          = "EncryptionNone"
	EncryptionEmpty         = ""
)

// MakeUser makes a VLESS user.
func MakeUser(id, flow, encryption string) *component.VlessUser {
	return &component.VlessUser{
		Id:         id,
		Flow:       flow,
		Encryption: encryption,
	}
}

// MakeVtrInbound makes a VLESS/TCP/Reality inbound.
func MakeVtrInbound(tag string, port int, privateKey string, clients []*component.VlessUser) *component.Inbound {
	return &component.Inbound{
		Tag:      tag,
		Port:     port,
		Protocol: Protocol,
		Settings: &component.InboundSettings{
			Clients:    clients,
			Decryption: EncryptionNone,
		},
		StreamSettings: &component.StreamSettings{
			Network:  NetworkTcp,
			Security: SecurityReality,
			RealitySettings: &component.RealitySettings{
				Dest:        DestStackOverflow,
				PrivateKey:  privateKey,
				ServerNames: []string{ServerNameStackOverflow},
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

// MakeVtrOutbound makes a VLESS/TCP/Reality outbound.
func MakeVtrOutbound(
	tag string,
	address string,
	port int,
	id string,
	publicKey string,
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
			Network:  NetworkTcp,
			Security: SecurityReality,
			RealitySettings: &component.RealitySettings{
				Fingerprint: "chrome",
				ServerName:  ServerNameStackOverflow,
				PublicKey:   publicKey,
				ShortId:     "",
			},
		},
	}
}
