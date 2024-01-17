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
	"strconv"
	"sync"
	"time"
	"xray-manager/pkg/utils"
)

type Xray struct {
	Config     *Config
	configPath string
	binaryPath string
	command    *exec.Cmd
	log        *zap.Logger
	connection *grpc.ClientConn
	lock       sync.Mutex
}

// initConfig stores init configurations if there is no Config file and loads it.
func (x *Xray) initConfig() {
	if !utils.FileExist(x.configPath) {
		x.saveConfig()
	}
	x.loadConfig()
}

// loadConfig loads the stored configuration from file.
func (x *Xray) loadConfig() {
	x.lock.Lock()
	defer x.lock.Unlock()

	content, err := os.ReadFile(x.configPath)
	if err != nil {
		x.log.Fatal("xray: cannot load Config file", zap.Error(err))
	}

	err = json.Unmarshal(content, x.Config)
	if err != nil {
		x.log.Fatal("xray: cannot unmarshal Config file", zap.Error(err))
	}

	if err = validator.New().Struct(x); err != nil {
		x.log.Fatal("xray: cannot validate Config file", zap.Error(err))
	}
}

// saveConfig saves the current configurations.
func (x *Xray) saveConfig() {
	defer func() {
		x.loadConfig()
	}()
	content, err := json.Marshal(x.Config)
	if err != nil {
		x.log.Fatal("xray: cannot marshal Config", zap.Error(err))
	}

	x.lock.Lock()
	defer x.lock.Unlock()

	if err = os.WriteFile(x.configPath, content, 0755); err != nil {
		x.log.Fatal("xray: cannot save Config", zap.String("file", x.configPath), zap.Error(err))
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
	index := x.Config.findApiInboundIndex()
	if index == -1 {
		x.log.Fatal("xray: cannot find api inbound")
	}

	op := x.Config.Inbounds[index].Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.log.Fatal("xray: cannot find free port for api inbound", zap.Error(err))
		}
		x.log.Debug("xray: updating api inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.Config.Inbounds[index].Port = np
		x.saveConfig()
	}
}

// runCore runs Xray core.
func (x *Xray) runCore() {
	x.command = exec.Command(x.binaryPath, "-c", x.configPath)
	x.command.Stderr = os.Stderr
	x.command.Stdout = os.Stdout

	x.log.Debug("xray: starting the xray core...")
	if err := x.command.Run(); err != nil && err.Error() != "signal: killed" {
		x.log.Fatal("xray: cannot start the xray core", zap.Error(err))
	}
}

func (x *Xray) UpdateInbounds(inbounds []Inbound) {
	inbounds = append(inbounds, x.Config.Inbounds[x.Config.findApiInboundIndex()])
	x.Config.Inbounds = inbounds
	x.saveConfig()
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

// connectGrpc connects to the GRPC APIs provided by Xray core.
func (x *Xray) connectGrpc() {
	x.log.Debug("xray: connecting to xray core grpc...")

	index := x.Config.findApiInboundIndex()
	if index == -1 {
		x.log.Fatal("xray: cannot find api inbound")
	}

	port := x.Config.Inbounds[index].Port
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
func New(l *zap.Logger, configPath, binaryPath string) *Xray {
	return &Xray{log: l, Config: NewConfig(), binaryPath: binaryPath, configPath: configPath}
}
