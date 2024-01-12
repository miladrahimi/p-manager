package config

import (
	"encoding/json"
	"fmt"
	"os"
	"shadowsocks-manager/internal/utils"
)

const MainPath = "configs/main.json"
const LocalPath = "configs/main.local.json"
const AppName = "ShadowsocksManager"
const AppVersion = "v1.0.0"
const ShadowsocksMethod = "chacha20-ietf-poly1305"

// Config is the root configuration.
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
	if utils.FileExist(LocalPath) {
		content, err = os.ReadFile(LocalPath)
	} else {
		content, err = os.ReadFile(MainPath)
	}
	if err != nil {
		return fmt.Errorf("config: cannot load file, err: %v", err)
	}

	err = json.Unmarshal(content, &c)
	if err != nil {
		return fmt.Errorf("config: cannot validate file, err: %v", err)
	}

	return nil
}

// New creates an instance of the Config.
func New() *Config {
	return &Config{}
}
