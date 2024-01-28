package xray

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/miladrahimi/xray-manager/internal/config"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"slices"
	"strconv"
	"sync"
)

type Log struct {
	LogLevel string `json:"loglevel" validate:"required,oneof=debug warning"`
}

type Client struct {
	Password string `json:"password" validate:"required,min=1,max=64"`
	Method   string `json:"method" validate:"required,oneof=chacha20-ietf-poly1305 aes-128-gcm aes-256-gcm"`
	Email    string `json:"email" validate:"required,number"`
}

type InboundSettings struct {
	Address  string    `json:"address,omitempty"`
	Clients  []*Client `json:"clients,omitempty" validate:"omitempty,dive"`
	Network  string    `json:"network,omitempty"`
	Method   string    `json:"method,omitempty" validate:"omitempty"`
	Password string    `json:"password,omitempty" validate:"omitempty"`
	Port     int       `json:"port,omitempty" validate:"omitempty,min=1,max=65536"`
}

type Inbound struct {
	Listen   string           `json:"listen" validate:"required,oneof=127.0.0.1 0.0.0.0"`
	Port     int              `json:"port" validate:"required,min=1,max=65536"`
	Protocol string           `json:"protocol" validate:"required,oneof=shadowsocks dokodemo-door"`
	Settings *InboundSettings `json:"settings" validate:"required"`
	Tag      string           `json:"tag" validate:"required"`
}

type OutboundServer struct {
	Address  string `json:"address" validate:"required"`
	Port     int    `json:"port" validate:"required,min=1,max=65536"`
	Method   string `json:"method" validate:"required,oneof=2022-blake3-aes-256-gcm"`
	Password string `json:"password" validate:"required"`
}

type OutboundSettings struct {
	Servers []*OutboundServer `json:"servers" validate:"omitempty,dive"`
}

type StreamSettings struct {
	Network string `json:"network" validate:"required"`
}

type Outbound struct {
	Protocol       string            `json:"protocol" validate:"required,oneof=freedom shadowsocks"`
	Tag            string            `json:"tag" validate:"required"`
	Settings       *OutboundSettings `json:"settings,omitempty" validate:"omitempty"`
	StreamSettings *StreamSettings   `json:"streamSettings,omitempty" validate:"omitempty"`
}

type DNS struct {
	Servers []string `json:"servers" validate:"required"`
}

type API struct {
	Tag      string   `json:"tag" validate:"required"`
	Services []string `json:"services" validate:"required"`
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
	InboundTag  []string `json:"inboundTag" validate:"required"`
	OutboundTag string   `json:"outboundTag" validate:"required"`
	Type        string   `json:"type" validate:"required"`
	Domain      []string `json:"domain,omitempty" validate:"omitempty"`
}

type RoutingSettings struct {
	Rules []*Rule `json:"rules" validate:"required,dive"`
}

type Routing struct {
	DomainStrategy string           `json:"domainStrategy" validate:"required"`
	DomainMatcher  string           `json:"domainMatcher" validate:"required"`
	Strategy       string           `json:"strategy" validate:"required"`
	Settings       *RoutingSettings `json:"settings" validate:"required"`
}

type Reverse struct {
	Bridges []*ReverseItem `json:"bridges,omitempty"  validate:"omitempty,dive"`
	Portals []*ReverseItem `json:"portals,omitempty"  validate:"omitempty,dive"`
}

type ReverseItem struct {
	Tag    string `json:"tag"  validate:"required"`
	Domain string `json:"domain"  validate:"required"`
}

type Config struct {
	Log       *Log                   `json:"log" validate:"required"`
	Inbounds  []*Inbound             `json:"inbounds" validate:"required,dive"`
	Outbounds []*Outbound            `json:"outbounds" validate:"dive"`
	DNS       *DNS                   `json:"dns" validate:"required"`
	Stats     map[string]interface{} `json:"stats" validate:"required"`
	API       *API                   `json:"api" validate:"required"`
	Policy    *Policy                `json:"policy" validate:"required"`
	Routing   *Routing               `json:"routing" validate:"required"`
	Reverse   *Reverse               `json:"reverse" validate:"omitempty"`
	Locker    *sync.Mutex            `json:"-"`
}

func (c *Config) ApiInboundIndex() int {
	index := -1
	for i, inbound := range c.Inbounds {
		if inbound.Tag == "api" {
			index = i
		}
	}
	return index
}

func (c *Config) ApiInbound() *Inbound {
	return c.Inbounds[c.ApiInboundIndex()]
}

func (c *Config) UpdateApiInbound(port int) {
	index := c.ApiInboundIndex()
	if index == -1 {
		c.Inbounds = append(c.Inbounds, &Inbound{
			Tag:      "api",
			Protocol: "dokodemo-door",
			Listen:   "127.0.0.1",
			Port:     port,
			Settings: &InboundSettings{
				Address: "127.0.0.1",
				Network: "tcp",
			},
		})
	} else {
		c.Inbounds[index].Port = port
	}
}

func (c *Config) ShadowsocksInboundIndex() int {
	index := -1
	for i, inbound := range c.Inbounds {
		if inbound.Tag == "shadowsocks" {
			index = i
		}
	}
	return index
}

func (c *Config) ShadowsocksInbound() *Inbound {
	if c.ShadowsocksInboundIndex() != -1 {
		return c.Inbounds[c.ShadowsocksInboundIndex()]
	}
	return nil
}

func (c *Config) UpdateShadowsocksInbound(clients []*Client, port int) {
	index := c.ShadowsocksInboundIndex()
	if len(clients) > 0 {
		inbound := &Inbound{
			Tag:      "shadowsocks",
			Protocol: "shadowsocks",
			Listen:   "0.0.0.0",
			Port:     port,
			Settings: &InboundSettings{
				Clients:  clients,
				Network:  "tcp,udp",
				Password: utils.GenerateKey32(),
				Method:   config.ShadowsocksMethod,
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

func (c *Config) ReverseInboundIndex() int {
	index := -1
	for i, inbound := range c.Inbounds {
		if inbound.Tag == "reverse" {
			index = i
		}
	}
	return index
}

func (c *Config) ReverseInbound() *Inbound {
	if c.ReverseInboundIndex() != -1 {
		return c.Inbounds[c.ReverseInboundIndex()]
	}
	return nil
}

func (c *Config) UpdateReverseInbound(port int, password string) {
	index := c.ReverseInboundIndex()
	if index == -1 {
		c.Inbounds = append(c.Inbounds, &Inbound{
			Tag:      "reverse",
			Protocol: "shadowsocks",
			Listen:   "0.0.0.0",
			Port:     port,
			Settings: &InboundSettings{
				Method:   config.Shadowsocks2022Method,
				Password: password,
				Network:  "tcp,udp",
			},
		})
	} else {
		c.Inbounds[index].Port = port
		c.Inbounds[index].Settings.Password = password
	}
}

func (c *Config) ReverseOutboundIndex() int {
	index := -1
	for i, outbound := range c.Outbounds {
		if outbound.Tag == "reverse" {
			index = i
		}
	}
	return index
}

func (c *Config) ReverseOutbound() *Outbound {
	if c.ReverseOutboundIndex() != -1 {
		return c.Outbounds[c.ReverseOutboundIndex()]
	}
	return nil
}

func (c *Config) UpdateReverseOutbound(address string, port int, password string) {
	index := c.ReverseOutboundIndex()
	if index == -1 {
		c.Outbounds = append(c.Outbounds, &Outbound{
			Tag:      "reverse",
			Protocol: "shadowsocks",
			Settings: &OutboundSettings{
				Servers: []*OutboundServer{
					{
						Address:  address,
						Port:     port,
						Method:   config.Shadowsocks2022Method,
						Password: password,
					},
				},
			},
			StreamSettings: &StreamSettings{
				Network: "tcp",
			},
		})
	} else {
		c.Outbounds[index].Settings.Servers[0].Address = address
		c.Outbounds[index].Settings.Servers[0].Port = port
		c.Outbounds[index].Settings.Servers[0].Password = password
	}
}

func (c *Config) RemoveInbounds() {
	c.Inbounds = []*Inbound{c.ApiInbound()}
}

func (c *Config) AddRelayInbound(id int, host string, localPort, remotePort int) {
	c.Inbounds = append(c.Inbounds, &Inbound{
		Tag:      "relay-" + strconv.Itoa(id),
		Protocol: "dokodemo-door",
		Listen:   "0.0.0.0",
		Port:     localPort,
		Settings: &InboundSettings{
			Address: host,
			Port:    remotePort,
		},
	})
}

func (c *Config) Validate() error {
	v := validator.New(validator.WithRequiredStructEnabled())
	if err := v.Struct(c); err != nil {
		return err
	}
	if c.ApiInboundIndex() == -1 {
		return fmt.Errorf("api inbound not found")
	}
	return nil
}

func newEmptyConfig() *Config {
	return &Config{
		Locker: &sync.Mutex{},
	}
}

func NewPortalConfig() *Config {
	c := NewConfig()
	c.Reverse.Portals = []*ReverseItem{{Tag: "portal", Domain: "s1.google.com"}}
	c.Routing.Settings.Rules = append(c.Routing.Settings.Rules, []*Rule{
		{
			Type:        "field",
			InboundTag:  []string{"shadowsocks"},
			OutboundTag: "portal",
		},
		{
			Type:        "field",
			InboundTag:  []string{"reverse"},
			OutboundTag: "portal",
		},
	}...)
	c.Inbounds = append(c.Inbounds, []*Inbound{
		{
			Tag:      "shadowsocks",
			Protocol: "shadowsocks",
			Listen:   "0.0.0.0",
			Port:     2929,
			Settings: &InboundSettings{
				Method:   config.ShadowsocksMethod,
				Password: utils.GenerateKey32(),
				Network:  "tcp,udp",
				Clients: []*Client{
					{
						Password: utils.GenerateKey32(),
						Method:   config.ShadowsocksMethod,
						Email:    "1",
					},
				},
			},
		},
		{
			Tag:      "reverse",
			Protocol: "shadowsocks",
			Listen:   "0.0.0.0",
			Port:     2829,
			Settings: &InboundSettings{
				Method:   config.Shadowsocks2022Method,
				Password: utils.GenerateKey32(),
				Network:  "tcp,udp",
			},
		},
	}...)
	return c
}

func NewBridgeConfig() *Config {
	c := NewConfig()
	c.Reverse.Bridges = []*ReverseItem{{Tag: "bridge", Domain: "s1.google.com"}}
	c.Routing.Settings.Rules = append(c.Routing.Settings.Rules, []*Rule{
		{
			Type:        "field",
			InboundTag:  []string{"bridge"},
			Domain:      []string{"full:s1.google.com"},
			OutboundTag: "reverse",
		},
		{
			Type:        "field",
			InboundTag:  []string{"bridge"},
			OutboundTag: "freedom",
		},
	}...)
	c.Outbounds = []*Outbound{
		{
			Tag:      "reverse",
			Protocol: "shadowsocks",
			Settings: &OutboundSettings{
				Servers: []*OutboundServer{
					{
						Address:  "127.0.0.1",
						Port:     2929,
						Method:   config.Shadowsocks2022Method,
						Password: utils.GenerateKey32(),
					},
				},
			},
			StreamSettings: &StreamSettings{
				Network: "tcp",
			},
		},
		{
			Tag:      "freedom",
			Protocol: "freedom",
		},
	}
	return c
}

func NewConfig() *Config {
	return &Config{
		Locker: &sync.Mutex{},
		Log: &Log{
			LogLevel: "warning",
		},
		Inbounds: []*Inbound{
			{
				Tag:      "api",
				Protocol: "dokodemo-door",
				Listen:   "127.0.0.1",
				Port:     3411,
				Settings: &InboundSettings{
					Address: "127.0.0.1",
					Network: "tcp",
				},
			},
		},
		DNS: &DNS{
			Servers: []string{"8.8.8.8", "8.8.4.4", "localhost"},
		},
		Stats: map[string]interface{}{},
		API: &API{
			Tag:      "api",
			Services: []string{"StatsService"},
		},
		Policy: &Policy{
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
		Routing: &Routing{
			DomainStrategy: "AsIs",
			DomainMatcher:  "hybrid",
			Strategy:       "rules",
			Settings: &RoutingSettings{
				Rules: []*Rule{
					{
						Type:        "field",
						InboundTag:  []string{"api"},
						OutboundTag: "api",
					},
				},
			},
		},
		Reverse: &Reverse{},
	}
}
