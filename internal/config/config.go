package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/miladrahimi/p-node/pkg/util"
)

const AppName = "P-Manager"
const AppVersion = "v26.9.6"
const CoreName = "Xray-core v26.3.27"
const MaxAccountsCount = 1024
const MaxNodesCount = 10
const DefaultNodeSni = "dl.google.com"
const DefaultManagerSni = "snapp.ir"

// Config represents the application configuration.
type Config struct {
	HttpServer struct {
		Host string `json:"host" validate:"required,ip"`
		Port int    `json:"port" validate:"required,min=1,max=65535"`
	} `json:"http_server" validate:"required"`

	HttpClient struct {
		Timeout int `json:"timeout" validate:"required,min=10,max=60000"`
	} `json:"http_client" validate:"required"`

	Logger struct {
		Level  string `json:"level" validate:"required,oneof=debug info warn error"`
		Format string `json:"format" validate:"required,oneof='2006-01-02 15:04:05.000'"`
	} `json:"logger" validate:"required"`

	Xray struct {
		LogLevel string `json:"log_level" validate:"required,oneof=debug info warning error none"`
	} `json:"xray" validate:"required"`

	// Paqet overrides the network paqet sends raw packets on (Relay RR2PQ).
	// Leave all three empty to detect the default-route interface, its IPv4
	// address, and the gateway MAC at runtime.
	Paqet PaqetNetwork `json:"paqet"`

	// Cloudflare overrides the API base URL of Remote CXH (tests); empty
	// means the real API.
	Cloudflare struct {
		BaseUrl string `json:"base_url" validate:"omitempty,url"`
	} `json:"cloudflare"`
}

// PaqetNetwork is the optional paqet network override.
type PaqetNetwork struct {
	Interface string `json:"interface" validate:"max=15"`
	Address   string `json:"address" validate:"omitempty,ipv4"`
	RouterMac string `json:"router_mac" validate:"omitempty,mac"`
}

// Set reports whether the override is set (all three fields).
func (p PaqetNetwork) Set() bool {
	return p.Interface != "" && p.Address != "" && p.RouterMac != ""
}

// New creates a new instance of Config and loads the default and local config files.
func New(root string) (*Config, error) {
	c := &Config{}

	content, err := os.ReadFile(defaultConfigPath(root))
	if err != nil {
		return c, errors.WithStack(err)
	}
	err = json.Unmarshal(content, c)
	if err != nil {
		return c, errors.WithStack(err)
	}

	lcp := localConfigPath(root)
	if util.FileExist(lcp) {
		content, err = os.ReadFile(lcp)
		if err != nil {
			return c, errors.WithStack(err)
		}
		if err = json.Unmarshal(content, c); err != nil {
			return c, errors.WithStack(err)
		}

		var contentBytes []byte
		contentBytes, err = json.MarshalIndent(c, "", "  ")
		if err != nil {
			return c, errors.WithStack(err)
		}
		if err = os.WriteFile(lcp, contentBytes, 0644); err != nil {
			return c, errors.WithStack(err)
		}
	}

	fmt.Println("Config:", c.String())

	return c, errors.WithStack(validator.New().Struct(c))
}

// String returns a string representation of the configuration.
func (c *Config) String() string {
	j, err := json.Marshal(c)
	if err != nil {
		return err.Error()
	}
	return string(j)
}
