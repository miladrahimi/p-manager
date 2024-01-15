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

// binaryPath returns the path of Xray core binary for current OS.
func (x *Xray) binaryPath() string {
	if path, found := binaryPaths[runtime.GOOS]; found {
		return path
	}
	return binaryPaths["linux"]
}

// initConfig stores init configurations if there is no config file and loads it.
func (x *Xray) initConfig() {
	if !utils.FileExist(configPath) {
		x.saveConfig()
	}
	x.loadConfig()
}

// loadConfig loads the stored configuration from file.
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

// saveConfig saves the current configurations.
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

// Run prepare and starts the Xray core process.
func (x *Xray) Run() {
	x.initConfig()
	x.initApiPort()
	go x.runCore()
	x.connectGrpc()
}

// initApiPort finds a free port for api inbound.
func (x *Xray) initApiPort() {
	index := x.findApiInboundIndex()
	op := x.config.Inbounds[index].Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.log.Fatal("xray: cannot find free port for api inbound", zap.Error(err))
		}
		x.log.Debug("xray: updating api inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.config.Inbounds[index].Port = np
		x.saveConfig()
	}
}

// runCore runs Xray core.
func (x *Xray) runCore() {
	x.command = exec.Command(x.binaryPath(), "-c", configPath)
	x.command.Stderr = os.Stderr
	x.command.Stdout = os.Stdout

	x.log.Debug("xray: starting the xray core...")
	if err := x.command.Run(); err != nil && err.Error() != "signal: killed" {
		x.log.Fatal("xray: cannot start the xray core", zap.Error(err))
	}
}

// UpdateShadowsocksInboundPort updates the shadowsocks inbound port.
func (x *Xray) UpdateShadowsocksInboundPort(port int) {
	x.log.Debug("xray: updating shadowsocks inbound port...", zap.Int("port", port))

	index := x.findShadowsocksInboundIndex()
	if x.config.Inbounds[index].Port != port {
		x.config.Inbounds[index].Port = port
		x.saveConfig()
		x.Restart()
	}
}

// UpdateClients updates the shadowsocks inbound clients (users).
func (x *Xray) UpdateClients(clients []Client) {
	x.log.Debug("xray: updating clients...", zap.Int("count", len(clients)))

	index := x.findShadowsocksInboundIndex()
	x.config.Inbounds[index].Settings.Clients = clients

	x.saveConfig()
	x.Restart()
}

// UpdateServers updates the outbound servers.
func (x *Xray) UpdateServers(servers []Server) {
	x.log.Debug("xray: updating servers...", zap.Int("count", len(servers)))

	x.config.Outbounds[0].Settings.Servers = servers

	x.saveConfig()
	x.Restart()
}

// Restart closes and runs the Xray core.
func (x *Xray) Restart() {
	x.log.Info("xray: restarting the xray core...")
	x.Shutdown()
	x.Run()
}

// Shutdown closes Xray core process.
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

// findApiInboundIndex finds the index of the api inbound.
func (x *Xray) findApiInboundIndex() int {
	index := -1
	for i, inbound := range x.config.Inbounds {
		if inbound.Tag == "api" {
			index = i
		}
	}
	if index == -1 {
		x.log.Fatal("xray: api tag not found")
	}
	return index
}

// findShadowsocksInboundIndex finds the index of the shadowsocks inbound.
func (x *Xray) findShadowsocksInboundIndex() int {
	index := -1
	for i, inbound := range x.config.Inbounds {
		if inbound.Tag == "shadowsocks" {
			index = i
		}
	}
	if index == -1 {
		x.log.Fatal("xray: shadowsocks tag not found")
	}
	return index
}

// connectGrpc connects to the GRPC APIs provided by Xray core.
func (x *Xray) connectGrpc() {
	x.log.Debug("xray: connecting to xray core grpc...")

	port := x.config.Inbounds[x.findApiInboundIndex()].Port
	address := "127.0.0.1:" + strconv.Itoa(port)
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

// QueryStats fetches the traffic stats from Xray core.
func (x *Xray) QueryStats() ([]*stats.Stat, error) {
	client := stats.NewStatsServiceClient(x.connection)
	qs, err := client.QueryStats(context.Background(), &stats.QueryStatsRequest{Reset_: true})
	if err != nil {
		return nil, err
	}
	return qs.GetStat(), nil
}

// New creates a new instance of Xray.
func New(l *zap.Logger) *Xray {
	return &Xray{log: l, config: NewConfig()}
}
