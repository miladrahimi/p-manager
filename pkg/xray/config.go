package xray

import (
	"fmt"
	"github.com/go-playground/validator/v10"
)

type Log struct {
	LogLevel string `json:"loglevel" validate:"required"`
	Access   string `json:"access,omitempty"`
	Error    string `json:"error,omitempty"`
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
	Balancers      []*Balancer      `json:"balancers,omitempty" validate:"omitempty,dive"`
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
	Reverse   *Reverse               `json:"reverse,omitempty"`
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

func (c *Config) FindBalancer(tag string) *Balancer {
	for _, balancer := range c.Routing.Balancers {
		if balancer.Tag == tag {
			return balancer
		}
	}
	return nil
}

func (c *Config) Validate() error {
	v := validator.New(validator.WithRequiredStructEnabled())
	if err := v.Struct(c); err != nil {
		return err
	}
	if c.FindInbound("api") == nil {
		return fmt.Errorf("api inbound not found")
	}
	return nil
}

func NewConfig() *Config {
	return &Config{
		Log: &Log{
			LogLevel: "warning",
			Access:   "./storage/logs/xray-access.log",
			Error:    "./storage/logs/xray-error.log",
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
		Outbounds: []*Outbound{
			{
				Tag:      "freedom",
				Protocol: "freedom",
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
			Balancers: []*Balancer{},
		},
		Reverse: &Reverse{
			Bridges: []*ReverseItem{},
			Portals: []*ReverseItem{},
		},
	}
}
