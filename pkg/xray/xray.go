package xray

import (
	"context"
	"encoding/json"
	"github.com/miladrahimi/xray-manager/pkg/logger"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	stats "github.com/xtls/xray-core/app/stats/command"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type Xray struct {
	config     *Config
	configPath string
	binaryPath string
	command    *exec.Cmd
	log        *logger.Logger
	connection *grpc.ClientConn
	locker     *sync.Mutex
}

func (x *Xray) initConfig() {
	x.config.Locker.Lock()
	defer x.config.Locker.Unlock()

	if !utils.FileExist(x.configPath) {
		x.saveConfig()
	} else {
		x.loadConfig()
	}
}

func (x *Xray) loadConfig() {
	content, err := os.ReadFile(x.configPath)
	if err != nil {
		x.log.Fatal("xray: cannot load Config file", zap.Error(err))
	}

	newConfig := newEmptyConfig()
	err = json.Unmarshal(content, newConfig)
	if err != nil {
		x.log.Fatal("xray: cannot unmarshal Config file", zap.Error(err))
	}

	if err = newConfig.Validate(); err != nil {
		x.log.Fatal("xray: cannot validate Config file", zap.Error(err))
	}

	x.config = newConfig
}

func (x *Xray) saveConfig() {
	defer func() {
		x.loadConfig()
	}()

	content, err := json.Marshal(x.config)
	if err != nil {
		x.log.Fatal("xray: cannot marshal Config", zap.Error(err))
	}

	if err = os.WriteFile(x.configPath, content, 0755); err != nil {
		x.log.Fatal("xray: cannot save Config", zap.String("file", x.configPath), zap.Error(err))
	}
}

func (x *Xray) Run() {
	x.locker.Lock()
	defer x.locker.Unlock()

	x.initConfig()
	x.initApiPort()
	go x.runCore()
	x.connectGrpc()
}

func (x *Xray) initApiPort() {
	x.config.Locker.Lock()
	defer x.config.Locker.Unlock()

	op := x.config.ApiInbound().Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.log.Fatal("xray: cannot find free port for api inbound", zap.Error(err))
		}
		x.log.Info("xray: updating api inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.config.UpdateApiInbound(np)
		x.saveConfig()
	}
}

func (x *Xray) runCore() {
	if !utils.FileExist(x.binaryPath) {
		x.log.Fatal("xray: core binary file not found", zap.String("path", x.binaryPath))
	}

	x.command = exec.Command(x.binaryPath, "-c", x.configPath)
	x.command.Stderr = os.Stderr
	x.command.Stdout = os.Stdout

	x.log.Info("xray: starting the xray core...")
	if err := x.command.Run(); err != nil && err.Error() != "signal: killed" {
		x.log.Fatal("xray: cannot start the xray core", zap.Error(err))
	}
}

func (x *Xray) Restart() {
	x.log.Info("xray: restarting the xray core...")
	x.saveConfig()
	x.Shutdown()
	x.Run()
}

func (x *Xray) Shutdown() {
	x.locker.Lock()
	defer x.locker.Unlock()

	x.log.Info("xray: shutting down the xray core...")
	if x.connection != nil {
		_ = x.connection.Close()
	}
	if x.command != nil && x.command.Process != nil {
		if err := x.command.Process.Kill(); err != nil {
			x.log.Error("xray: failed to shutdown the xray core", zap.Error(err))
		} else {
			x.log.Info("xray: the xray core closed successfully")
		}
	} else {
		x.log.Info("xray: the xray core is already closed")
	}
}

func (x *Xray) connectGrpc() {
	x.log.Info("xray: connecting to xray core grpc...")

	index := x.config.ApiInboundIndex()
	if index == -1 {
		x.log.Fatal("xray: cannot find api inbound")
	}

	port := x.config.Inbounds[index].Port
	address := "127.0.0.1:" + strconv.Itoa(port)
	var err error
	for i := 0; i < 5; i++ {
		x.connection, err = grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			x.log.Debug("xray: trying to connect to grpc", zap.Int("try", i))
		} else {
			return
		}
		time.Sleep(time.Second)
	}

	x.log.Error("xray: cannot connect the xray core grpc", zap.Error(err))
}

func (x *Xray) SetConfig(config *Config) {
	x.config = config
}

func (x *Xray) Config() *Config {
	return x.config
}

func (x *Xray) QueryStats() []*stats.Stat {
	client := stats.NewStatsServiceClient(x.connection)
	qs, err := client.QueryStats(context.Background(), &stats.QueryStatsRequest{Reset_: true})
	if err != nil {
		x.log.Error("xray: cannot fetch query stats", zap.Error(err))
	}
	return qs.GetStat()
}

func New(l *logger.Logger, configPath, binaryPath string) *Xray {
	return &Xray{log: l, config: NewConfig(), binaryPath: binaryPath, configPath: configPath, locker: &sync.Mutex{}}
}
