package config

import (
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
	"github.com/miladrahimi/p-manager/pkg/utils"
	"os"
	"runtime"
)

const defaultConfigPath = "configs/main.defaults.json"
const envConfigPath = "configs/main.json"

const AppName = "P-Manager"
const AppVersion = "v1.4.1"
const CoreVersion = "Xray v1.8.8"

const ShadowsocksMethod = "chacha20-ietf-poly1305"
const Shadowsocks2022Method = "2022-blake3-aes-256-gcm"

const FreeUsersCount = 16
const MaxUsersCount = 256
const MaxActiveUsersCount = 128

const LicensePath = "storage/app/license.txt"
const EnigmaKeyPath = "assets/ed25519_public_key.txt"
const XrayConfigPath = "storage/app/xray.json"

var xrayBinaryPaths = map[string]string{
	"darwin": "third_party/xray-macos-arm64/xray",
	"linux":  "third_party/xray-linux-64/xray",
}

func XrayBinaryPath() string {
	if path, found := xrayBinaryPaths[runtime.GOOS]; found {
		return path
	}
	return xrayBinaryPaths["linux"]
}

type Config struct {
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
}

func (c *Config) String() string {
	j, err := json.Marshal(c)
	if err != nil {
		return err.Error()
	}
	return string(j)
}

func (c *Config) Init() (err error) {
	content, err := os.ReadFile(defaultConfigPath)
	if err != nil {
		return errors.WithStack(err)
	}
	err = json.Unmarshal(content, &c)
	if err != nil {
		return errors.WithStack(err)
	}

	if utils.FileExist(envConfigPath) {
		content, err = os.ReadFile(envConfigPath)
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

func New() *Config {
	return &Config{}
}
