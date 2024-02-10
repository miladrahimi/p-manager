package xray

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/miladrahimi/xray-manager/internal/config"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"sync"
)

type Log struct {
	LogLevel string `json:"loglevel" validate:"required"`
}

type Client struct {
	Password string `json:"password" validate:"required,min=1,max=64"`
	Method   string `json:"method" validate:"required"`
	Email    string `json:"email" validate:"required"`
}

type InboundSettings struct {
	Address  string    `json:"address,omitempty"`
	Clients  []*Client `json:"clients,omitempty" validate:"omitempty,dive"`
	Network  string    `json:"network,omitempty"`
	Method   string    `json:"method,omitempty"`
	Password string    `json:"password,omitempty"`
}

type Inbound struct {
	Listen   string           `json:"listen" validate:"required"`
	Port     int              `json:"port" validate:"required,min=1,max=65536"`
	Protocol string           `json:"protocol" validate:"required"`
	Settings *InboundSettings `json:"settings" validate:"required"`
	Tag      string           `json:"tag" validate:"required"`
}

type OutboundServer struct {
	Address  string `json:"address" validate:"required"`
	Port     int    `json:"port" validate:"required,min=1,max=65536"`
	Method   string `json:"method" validate:"required"`
	Password string `json:"password" validate:"required"`
	Uot      bool   `json:"uot"`
}

type OutboundSettings struct {
	Servers []*OutboundServer `json:"servers" validate:"omitempty,dive"`
}

type StreamSettings struct {
	Network string `json:"network" validate:"required"`
}

type Outbound struct {
	Protocol       string            `json:"protocol" validate:"required"`
	Tag            string            `json:"tag" validate:"required"`
	Settings       *OutboundSettings `json:"settings,omitempty"`
	StreamSettings *StreamSettings   `json:"streamSettings,omitempty"`
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
	OutboundTag string   `json:"outboundTag,omitempty"`
	BalancerTag string   `json:"balancerTag,omitempty"`
	Type        string   `json:"type" validate:"required"`
	Domain      []string `json:"domain,omitempty"`
}

type RoutingSettings struct {
	Rules []*Rule `json:"rules" validate:"required,dive"`
}

type Balancer struct {
	Tag      string   `json:"tag" validate:"required"`
	Selector []string `json:"selector"`
}

type Routing struct {
	DomainStrategy string           `json:"domainStrategy" validate:"required"`
	DomainMatcher  string           `json:"domainMatcher" validate:"required"`
	Strategy       string           `json:"strategy" validate:"required"`
	Settings       *RoutingSettings `json:"settings" validate:"required"`
	Balancers      []*Balancer      `json:"balancers" validate:"omitempty,dive"`
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
	Reverse   *Reverse               `json:"reverse"`
	Locker    *sync.Mutex            `json:"-"`
}

func (c *Config) MakeShadowsocksInbound(tag, password, method string, port int, clients []*Client) *Inbound {
	return &Inbound{
		Tag:      tag,
		Protocol: "shadowsocks",
		Listen:   "0.0.0.0",
		Port:     port,
		Settings: &InboundSettings{
			Clients:  clients,
			Password: password,
			Method:   method,
			Network:  "tcp,udp",
		},
	}
}

func (c *Config) MakeShadowsocksOutbound(tag, host, password, method string, port int) *Outbound {
	return &Outbound{
		Tag:      tag,
		Protocol: "shadowsocks",
		Settings: &OutboundSettings{
			Servers: []*OutboundServer{
				{
					Address:  host,
					Port:     port,
					Method:   method,
					Password: password,
					Uot:      true,
				},
			},
		},
		StreamSettings: &StreamSettings{
			Network: "tcp",
		},
	}
}

func (c *Config) FindInbound(tag string) *Inbound {
	for _, inbound := range c.Inbounds {
		if inbound.Tag == tag {
			return inbound
		}
	}
	return nil
}

func (c *Config) FindOutbound(tag string) *Outbound {
	for _, outbound := range c.Outbounds {
		if outbound.Tag == tag {
			return outbound
		}
	}
	return nil
}

func (c *Config) apiInboundIndex() int {
	index := -1
	for i, inbound := range c.Inbounds {
		if inbound.Tag == "api" {
			index = i
		}
	}
	return index
}

func (c *Config) ApiInbound() *Inbound {
	return c.Inbounds[c.apiInboundIndex()]
}

func (c *Config) directInboundIndex() int {
	index := -1
	for i, inbound := range c.Inbounds {
		if inbound.Tag == "direct" {
			index = i
		}
	}
	return index
}

func (c *Config) DirectInbound() *Inbound {
	if c.directInboundIndex() != -1 {
		return c.Inbounds[c.directInboundIndex()]
	}
	return nil
}

func (c *Config) foreignInboundIndex() int {
	index := -1
	for i, inbound := range c.Inbounds {
		if inbound.Tag == "foreign" {
			index = i
		}
	}
	return index
}

func (c *Config) ForeignInbound() *Inbound {
	if c.foreignInboundIndex() != -1 {
		return c.Inbounds[c.foreignInboundIndex()]
	}
	return nil
}

func (c *Config) ForeignInboundUpdate(port int, password string) {
	index := c.foreignInboundIndex()
	if index != -1 {
		c.Inbounds[index].Port = port
		c.Inbounds[index].Settings.Password = password
	}
}

func (c *Config) relayOutboundIndex() int {
	index := -1
	for i, outbound := range c.Outbounds {
		if outbound.Tag == "relay" {
			index = i
		}
	}
	return index
}

func (c *Config) RelayOutbound() *Outbound {
	if c.relayOutboundIndex() != -1 {
		return c.Outbounds[c.relayOutboundIndex()]
	}
	return nil
}

func (c *Config) RelayOutboundUpdate(servers []*OutboundServer) {
	index := c.relayOutboundIndex()
	if c.relayOutboundIndex() != -1 {
		c.Outbounds[index].Settings.Servers = servers
	}
}

func (c *Config) foreignOutboundIndex() int {
	index := -1
	for i, outbound := range c.Outbounds {
		if outbound.Tag == "foreign" {
			index = i
		}
	}
	return index
}

func (c *Config) ForeignOutbound() *Outbound {
	if c.foreignOutboundIndex() != -1 {
		return c.Outbounds[c.foreignOutboundIndex()]
	}
	return nil
}

func (c *Config) ForeignOutboundUpdate(address string, port int, password string) {
	index := c.foreignOutboundIndex()
	if index != -1 {
		c.Outbounds[index].Settings.Servers[0].Address = address
		c.Outbounds[index].Settings.Servers[0].Port = port
		c.Outbounds[index].Settings.Servers[0].Password = password
	}
}

func (c *Config) Validate() error {
	v := validator.New(validator.WithRequiredStructEnabled())
	if err := v.Struct(c); err != nil {
		return err
	}
	if c.ApiInbound() == nil {
		return fmt.Errorf("api inbound not found")
	}
	return nil
}

func newEmptyConfig() *Config {
	return &Config{
		Locker: &sync.Mutex{},
	}
}

func NewBridgeConfig() *Config {
	c := NewConfig()
	c.Reverse.Bridges = []*ReverseItem{{Tag: "bridge", Domain: "s1.google.com"}}
	c.Inbounds = append(c.Inbounds, []*Inbound{
		{
			Tag:      "direct",
			Protocol: "shadowsocks",
			Listen:   "0.0.0.0",
			Port:     1234,
			Settings: &InboundSettings{
				Clients:  []*Client{},
				Password: utils.Key32(),
				Method:   config.Shadowsocks2022Method,
			},
		},
	}...)
	c.Outbounds = append(c.Outbounds, []*Outbound{
		{
			Tag:      "foreign",
			Protocol: "shadowsocks",
			Settings: &OutboundSettings{
				Servers: []*OutboundServer{
					{
						Address:  "127.0.0.1",
						Port:     2929,
						Method:   config.Shadowsocks2022Method,
						Password: utils.Key32(),
						Uot:      true,
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
	}...)
	c.Routing.Settings.Rules = append(c.Routing.Settings.Rules, []*Rule{
		{
			Type:        "field",
			InboundTag:  []string{"bridge"},
			Domain:      []string{"full:s1.google.com"},
			OutboundTag: "foreign",
		},
		{
			Type:        "field",
			InboundTag:  []string{"bridge"},
			OutboundTag: "freedom",
		},
		{
			Type:        "field",
			InboundTag:  []string{"direct"},
			OutboundTag: "freedom",
		},
	}...)
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
		Outbounds: []*Outbound{},
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
			Balancers: []*Balancer{},
		},
		Reverse: &Reverse{
			Bridges: []*ReverseItem{},
			Portals: []*ReverseItem{},
		},
	}
}
