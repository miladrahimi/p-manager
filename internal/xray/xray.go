package xray

import (
	"context"
	"encoding/json"
	"github.com/go-playground/validator"
	stats "github.com/xtls/xray-core/app/stats/command"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"os/exec"
	"runtime"
	"shadowsocks-manager/internal/utils"
	"strconv"
	"sync"
	"time"
)

var configPath = "storage/xray.json"
var binaryPaths = map[string]string{
	"darwin": "third_party/xray-macos-arm64/xray",
	"linux":  "third_party/xray-linux-64/xray",
}

type Xray struct {
	command    *exec.Cmd
	log        *zap.Logger
	connection *grpc.ClientConn
	config     *Config
	lock       sync.Mutex
}

func (x *Xray) binaryPath() string {
	if path, found := binaryPaths[runtime.GOOS]; found {
		return path
	}
	return binaryPaths["linux"]
}

func (x *Xray) initConfig() {
	if !utils.FileExist(configPath) {
		x.saveConfig()
	}
	x.loadConfig()
}

func (x *Xray) loadConfig() {
	x.lock.Lock()
	defer x.lock.Unlock()

	content, err := os.ReadFile(configPath)
	if err != nil {
		x.log.Fatal("xray: cannot load config file", zap.Error(err))
	}

	err = json.Unmarshal(content, x.config)
	if err != nil {
		x.log.Fatal("xray: cannot unmarshal config file", zap.Error(err))
	}

	if err = validator.New().Struct(x); err != nil {
		x.log.Fatal("xray: cannot validate config file", zap.Error(err))
	}
}

func (x *Xray) saveConfig() {
	defer func() {
		x.loadConfig()
	}()
	content, err := json.Marshal(x.config)
	if err != nil {
		x.log.Fatal("xray: cannot marshal config", zap.Error(err))
	}

	x.lock.Lock()
	defer x.lock.Unlock()

	if err = os.WriteFile(configPath, content, 0755); err != nil {
		x.log.Fatal("xray: cannot save config", zap.String("file", configPath), zap.Error(err))
	}
}

func (x *Xray) Run() {
	x.initConfig()
	go x.runCore()
	x.connectGrpc()
}

func (x *Xray) runCore() {
	x.command = exec.Command(x.binaryPath(), "-c", configPath)
	x.command.Stderr = os.Stderr
	x.command.Stdout = os.Stdout

	x.log.Debug("xray: starting the xray core...")
	if err := x.command.Run(); err != nil && err.Error() != "signal: killed" {
		x.log.Fatal("xray: cannot start the xray core", zap.Error(err))
	}
}

func (x *Xray) UpdateInboundPort(port int) {
	x.log.Debug("xray: updating inbound port...", zap.Int("port", port))

	var inbound *Inbound
	for _, i := range x.config.Inbounds {
		if i.Tag == "shadowsocks" {
			inbound = &i
		}
	}
	if inbound == nil {
		x.log.Fatal("xray: shadowsocks tag not found")
	}

	inbound.Port = port
	x.saveConfig()
	x.Reconfigure()
}

func (x *Xray) UpdateClients(clients []Client) {
	x.log.Debug("xray: updating clients...", zap.Int("count", len(clients)))

	index := -1
	for i, inbound := range x.config.Inbounds {
		if inbound.Tag == "shadowsocks" {
			index = i
		}
	}
	if index == -1 {
		x.log.Fatal("xray: shadowsocks tag not found")
	}

	x.config.Inbounds[index].Settings.Clients = clients

	x.saveConfig()
	x.Reconfigure()
}

func (x *Xray) UpdateServers(servers []Server) {
	x.log.Debug("xray: updating servers...", zap.Int("count", len(servers)))

	x.config.Outbounds[0].Settings.Servers = servers

	x.saveConfig()
	x.Reconfigure()
}

func (x *Xray) Reconfigure() {
	x.log.Info("xray: reconfiguring the xray core...")
	x.Shutdown()
	x.Run()
}

func (x *Xray) Shutdown() {
	x.log.Debug("xray: shutting down the xray core...")
	if x.connection != nil {
		_ = x.connection.Close()
	}
	if x.command.Process != nil {
		if err := x.command.Process.Kill(); err != nil {
			x.log.Error("xray: failed to shutdown the xray core", zap.Error(err))
		} else {
			x.log.Debug("xray: the xray core stopped successfully")
		}
	} else {
		x.log.Debug("xray: the xray core is already stopped")
	}
}

func (x *Xray) connectGrpc() {
	x.log.Debug("xray: connecting to xray core grpc...")

	index := -1
	for i, inbound := range x.config.Inbounds {
		if inbound.Tag == "api" {
			index = i
		}
	}
	if index == -1 {
		x.log.Fatal("xray: api tag not found")
	}

	address := "127.0.0.1:" + strconv.Itoa(x.config.Inbounds[index].Port)
	var err error

	for i := 0; i < 5; i++ {
		x.connection, err = grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			x.log.Debug("xray: cannot connect the xray core grpc", zap.Int("try", i))
		} else {
			return
		}
		time.Sleep(time.Second)
	}

	x.log.Debug("xray: cannot connect the xray core grpc", zap.Error(err))
}

func (x *Xray) QueryStats() ([]*stats.Stat, error) {
	client := stats.NewStatsServiceClient(x.connection)
	qs, err := client.QueryStats(context.Background(), &stats.QueryStatsRequest{Reset_: true})
	if err != nil {
		return nil, err
	}
	return qs.GetStat(), nil
}

func New(l *zap.Logger) *Xray {
	return &Xray{log: l, config: NewConfig()}
}
