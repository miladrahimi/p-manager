package config

import (
	"encoding/json"
	"fmt"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"os"
	"runtime"
)

const MainPath = "configs/main.json"
const LocalPath = "configs/main.local.json"
const AppName = "XrayManager"
const AppVersion = "v1.2.3"
const ShadowsocksMethod = "chacha20-ietf-poly1305"
const Shadowsocks2022Method = "2022-blake3-aes-256-gcm"

const LimitedUsersCount = 200
const LicensePath = "storage/app/license.txt"
const EnigmaKeyPath = "assets/ed25519_public_key.txt"
const XrayConfigPath = "storage/app/xray.json"

var xrayBinaryPaths = map[string]string{
	"darwin": "third_party/xray-macos-arm64/xray",
	"linux":  "third_party/xray-linux-64/xray",
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

func (c *Config) Init() (err error) {
	var content []byte
	var path string

	if utils.FileExist(LocalPath) {
		path = LocalPath
	} else {
		path = MainPath
	}

	content, err = os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("config: cannot load file, err: %v", err)
	}

	err = json.Unmarshal(content, &c)
	if err != nil {
		return fmt.Errorf("config: cannot validate file, err: %v", err)
	}

	if path == LocalPath {
		marshalled, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return fmt.Errorf("config: cannot marshall config, err: %v", err)
		}

		err = os.WriteFile(path, marshalled, 0755)
		if err != nil {
			return fmt.Errorf("config: cannot save file, err: %v", err)
		}
	}

	return nil
}

func (c *Config) XrayBinaryPath() string {
	if path, found := xrayBinaryPaths[runtime.GOOS]; found {
		return path
	}
	return xrayBinaryPaths["linux"]
}

func New() *Config {
	return &Config{}
}
