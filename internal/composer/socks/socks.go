package socks

import "github.com/miladrahimi/p-node/pkg/xray/config/component"

const Protocol = "socks"

// MakeOutbound creates a SOCKS outbound configuration.
func MakeOutbound(tag string, address string, port int) *component.Outbound {
	return &component.Outbound{
		Tag:      tag,
		Protocol: Protocol,
		Settings: &component.OutboundSettings{
			Servers: []*component.SocksOutboundServer{
				{
					Address: address,
					Port:    port,
				},
			},
		},
	}
}
