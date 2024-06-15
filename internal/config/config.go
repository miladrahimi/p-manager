package config

import (
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/miladrahimi/p-manager/internal/utils"
	"os"
)

const AppName = "P-Manager"
const AppVersion = "v1.5.3"
const CoreVersion = "Xray v1.8.8"

const ShadowsocksMethod = "chacha20-ietf-poly1305"
const Shadowsocks2022Method = "2022-blake3-aes-128-gcm"

const FreeUsersCount = 16
const MaxUsersCount = 256
const MaxActiveUsersCount = 128

const LicenseServer = "https://x.miladrahimi.com/p-manager/v1/servers"
const LicenseToken = "Unauthorized"

type Config struct {
	Env        *Env `json:"-"`
	HttpServer struct {
		Host string `json:"host" validate:"required,ip"`
		Port int    `json:"port" validate:"required,min=1,max=65536"`
	} `json:"http_server" validate:"required"`

	HttpClient struct {
		Timeout int `json:"timeout" validate:"required,min=10,max=60000"`
	} `json:"http_client" validate:"required"`

	Logger struct {
		Level  string `json:"level" validate:"required,oneof=debug info warn error"`
		Format string `json:"format" validate:"required,oneof='2006-01-02 15:04:05.000'"`
	} `json:"logger" validate:"required"`

	Worker struct {
		Interval int `json:"interval" validate:"required,min=10,max=60000"`
	} `json:"worker" validate:"required"`

	Xray struct {
		LogLevel string `json:"log_level" validate:"required,oneof=debug info warning error none"`
	} `json:"xray" validate:"required"`
}

func (c *Config) String() string {
	j, err := json.Marshal(c)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

func (c *Config) Init() (err error) {
	content, err := os.ReadFile(c.Env.DefaultConfigPath)
	if err != nil {
		return errors.WithStack(err)
	}
	err = json.Unmarshal(content, &c)
	if err != nil {
		return errors.WithStack(err)
	}

	if utils.FileExist(c.Env.LocalConfigPath) {
		content, err = os.ReadFile(c.Env.LocalConfigPath)
		if err != nil {
			return errors.WithStack(err)
		}
		if err = json.Unmarshal(content, &c); err != nil {
			return errors.WithStack(err)
		}

		var contentBytes []byte
		contentBytes, err = json.MarshalIndent(c, "", "  ")
		if err != nil {
			return errors.WithStack(err)
		}
		if err = os.WriteFile(c.Env.LocalConfigPath, contentBytes, 0755); err != nil {
			return errors.WithStack(err)
		}
	}

	fmt.Println("Config:", c.String())

	return errors.WithStack(validator.New().Struct(c))
}

func New(e *Env) *Config {
	return &Config{
		Env: e,
	}
}
