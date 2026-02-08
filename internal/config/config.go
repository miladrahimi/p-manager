package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/miladrahimi/p-manager/pkg/util"
)

const AppName = "P-Manager"
const AppVersion = "v26.2.8"
const CoreVersion = "xray v26.1.23"

const ShadowsocksMethod = "chacha20-ietf-poly1305"
const Shadowsocks2022Method = "2022-blake3-aes-128-gcm"

const MaxUsersCount = 1024

const DatabaseFilePath = "storage/database/data.json"
const DatabaseBackupPath = "storage/database/backup-%s.json"

const XrayConfigPath = "storage/app/xray.json"

const defaultConfigPath = "configs/main.defaults.json"
const localConfigPath = "configs/main.json"

var xrayBinaryPaths = map[string]string{
	"darwin": "third_party/xray-macos-arm64/xray",
	"linux":  "third_party/xray-linux-64/xray",
}

// XrayBinaryPath returns the path of the xray binary for the current OS.
func XrayBinaryPath() string {
	if path, found := xrayBinaryPaths[runtime.GOOS]; found {
		return path
	}
	return xrayBinaryPaths["linux"]
}

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
}

// New creates a new instance of Config and loads the default and local config files.
func New() (*Config, error) {
	c := &Config{}

	content, err := os.ReadFile(defaultConfigPath)
	if err != nil {
		return c, errors.WithStack(err)
	}
	err = json.Unmarshal(content, c)
	if err != nil {
		return c, errors.WithStack(err)
	}

	if util.FileExist(localConfigPath) {
		content, err = os.ReadFile(localConfigPath)
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
		if err = os.WriteFile(localConfigPath, contentBytes, 0644); err != nil {
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
