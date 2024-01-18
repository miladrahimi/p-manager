package xray

import (
	"slices"
)

type Log struct {
	LogLevel string `json:"loglevel"`
}

type Client struct {
	Password string `json:"password"`
	Method   string `json:"method"`
	Email    string `json:"email"`
}

type InboundSettings struct {
	Address string   `json:"address,omitempty"`
	Clients []Client `json:"clients,omitempty"`
	Network string   `json:"network,omitempty"`
}

type Inbound struct {
	Listen   string          `json:"listen"`
	Port     int             `json:"port"`
	Protocol string          `json:"protocol"`
	Settings InboundSettings `json:"settings"`
	Tag      string          `json:"tag"`
}

type Server struct {
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Method   string `json:"method"`
	Password string `json:"password"`
}

type OutboundSettings struct {
	Servers []Server `json:"servers"`
}

type Outbound struct {
	Protocol       string            `json:"protocol"`
	Settings       *OutboundSettings `json:"settings,omitempty"`
	StreamSettings *struct {
		Network string `json:"network"`
	} `json:"streamSettings,omitempty"`
	Tag string `json:"tag"`
}

type DNS struct {
	Servers []string `json:"servers"`
}

type API struct {
	Tag      string   `json:"tag"`
	Services []string `json:"services"`
}

type PolicyLevels struct {
	StatsUserUplink   bool `json:"statsUserUplink"`
	StatsUserDownlink bool `json:"statsUserDownlink"`
}

type Policy struct {
	Levels map[string]map[string]bool `json:"levels"`
	System map[string]bool            `json:"system"`
}

type Rule struct {
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
	Type        string   `json:"type"`
	IP          []string `json:"ip,omitempty"`
	Domain      []string `json:"domain,omitempty"`
}

type RoutingSettings struct {
	Rules []Rule `json:"rules"`
}

type Routing struct {
	DomainStrategy string          `json:"domainStrategy"`
	DomainMatcher  string          `json:"domainMatcher"`
	Strategy       string          `json:"strategy"`
	Settings       RoutingSettings `json:"settings"`
}

type Config struct {
	Log       Log                    `json:"log"`
	Inbounds  []Inbound              `json:"inbounds"`
	Outbounds []Outbound             `json:"outbounds"`
	DNS       DNS                    `json:"dns"`
	Stats     map[string]interface{} `json:"stats"`
	API       API                    `json:"api"`
	Policy    Policy                 `json:"policy"`
	Routing   Routing                `json:"routing"`
}

// findApiInboundIndex finds the index of the api inbound.
func (c *Config) findApiInboundIndex() int {
	index := -1
	for i, inbound := range c.Inbounds {
		if inbound.Tag == "api" {
			index = i
		}
	}
	return index
}

// findShadowsocksInboundIndex finds the index of the shadowsocks inbound.
func (c *Config) findShadowsocksInboundIndex() int {
	index := -1
	for i, inbound := range c.Inbounds {
		if inbound.Tag == "shadowsocks" {
			index = i
		}
	}
	return index
}

func (c *Config) FindApiInboundPort() int {
	index := c.findApiInboundIndex()
	return c.Inbounds[index].Port
}

func (c *Config) UpdateApiInboundPort(port int) {
	index := c.findApiInboundIndex()
	c.Inbounds[index].Port = port
}

func (c *Config) UpdateShadowsocksInbound(clients []Client, port int) {
	index := c.findShadowsocksInboundIndex()
	if len(clients) > 0 {
		inbound := Inbound{
			Tag:      "shadowsocks",
			Protocol: "shadowsocks",
			Listen:   "0.0.0.0",
			Port:     port,
			Settings: InboundSettings{
				Clients: clients,
				Network: "tcp,udp",
			},
		}
		if index != -1 {
			c.Inbounds[index] = inbound
		} else {
			c.Inbounds = append(c.Inbounds, inbound)
		}
	} else {
		if index != -1 {
			c.Inbounds = slices.Delete(c.Inbounds, index, index+1)
		}
	}
}

// NewConfig creates a new instance of Xray Config.
func NewConfig() *Config {
	return &Config{
		Log: Log{
			LogLevel: "warning",
		},
		Inbounds: []Inbound{
			{
				Protocol: "dokodemo-door",
				Listen:   "127.0.0.1",
				Port:     2401,
				Settings: InboundSettings{Address: "127.0.0.1"},
				Tag:      "api",
			},
		},
		Outbounds: []Outbound{
			{
				Protocol: "freedom",
				Tag:      "freedom",
			},
		},
		DNS: DNS{
			Servers: []string{"8.8.8.8", "8.8.4.4", "localhost"},
		},
		Stats: map[string]interface{}{},
		API: API{
			Tag:      "api",
			Services: []string{"StatsService"},
		},
		Policy: Policy{
			Levels: map[string]map[string]bool{
				"0": {
					"statsUserUplink":   true,
					"statsUserDownlink": true,
				},
			},
			System: map[string]bool{
				"statsInboundUplink":    true,
				"statsInboundDownlink":  true,
				"statsOutboundUplink":   true,
				"statsOutboundDownlink": true,
			},
		},
		Routing: Routing{
			DomainStrategy: "AsIs",
			DomainMatcher:  "hybrid",
			Strategy:       "rules",
			Settings: RoutingSettings{
				Rules: []Rule{
					{
						InboundTag:  []string{"api"},
						OutboundTag: "api",
						Type:        "field",
					},
				},
			},
		},
	}
}
