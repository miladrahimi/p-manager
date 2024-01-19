package xray

import (
	"fmt"
	"github.com/go-playground/validator/v10"
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
	Address string    `json:"address,omitempty"`
	Clients []*Client `json:"clients,omitempty" validate:"omitempty,dive"`
	Network string    `json:"network,omitempty"`
	Port    int       `json:"port,omitempty" validate:"omitempty,min=1,max=65536"`
}

type Inbound struct {
	Listen   string           `json:"listen" validate:"required,oneof=127.0.0.1 0.0.0.0"`
	Port     int              `json:"port" validate:"required,min=1,max=65536"`
	Protocol string           `json:"protocol" validate:"required,oneof=shadowsocks dokodemo-door"`
	Settings *InboundSettings `json:"settings" validate:"required"`
	Tag      string           `json:"tag" validate:"required"`
}

type Outbound struct {
	Protocol string `json:"protocol" validate:"required,oneof=freedom"`
	Tag      string `json:"tag" validate:"required"`
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

type Config struct {
	Log       *Log                   `json:"log" validate:"required"`
	Inbounds  []*Inbound             `json:"inbounds" validate:"required,dive"`
	Outbounds []*Outbound            `json:"outbounds" validate:"required,dive"`
	DNS       *DNS                   `json:"dns" validate:"required"`
	Stats     map[string]interface{} `json:"stats" validate:"required"`
	API       *API                   `json:"api" validate:"required"`
	Policy    *Policy                `json:"policy" validate:"required"`
	Routing   *Routing               `json:"routing" validate:"required"`
	Locker    *sync.Mutex            `json:"-"`
}

// ApiInboundIndex finds the index of the api inbound.
func (c *Config) ApiInboundIndex() int {
	index := -1
	for i, inbound := range c.Inbounds {
		if inbound.Tag == "api" {
			index = i
		}
	}
	return index
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
		},
	}
}
