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
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"http_server"`

	HttpClient struct {
		Timeout int `json:"timeout"`
	} `json:"http_client"`

	Logger struct {
		Level  string `json:"level"`
		Format string `json:"format"`
	} `json:"logger"`

	Worker struct {
		Interval int `json:"interval"`
	} `json:"worker"`

	Xray struct {
		LogLevel string `json:"log_level"`
	} `json:"xray"`
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
	}

	fmt.Println("Config:", c.String())

	return errors.WithStack(validator.New().Struct(c))
}

func New(e *Env) *Config {
	return &Config{
		Env: e,
	}
}
