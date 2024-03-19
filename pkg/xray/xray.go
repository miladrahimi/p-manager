package xray

import (
	"context"
	"encoding/json"
	"github.com/miladrahimi/p-manager/pkg/logger"
	"github.com/miladrahimi/p-manager/pkg/utils"
	stats "github.com/xtls/xray-core/app/stats/command"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"os/exec"
	"strconv"
	"time"
)

type Xray struct {
	config     *Config
	configPath string
	binaryPath string
	command    *exec.Cmd
	l          *logger.Logger
	connection *grpc.ClientConn
}

func (x *Xray) loadConfig() {
	if !utils.FileExist(x.configPath) {
		return
	}

	content, err := os.ReadFile(x.configPath)
	if err != nil {
		x.l.Fatal("xray: cannot load Config file", zap.Error(err))
	}

	var newConfig Config
	err = json.Unmarshal(content, &newConfig)
	if err != nil {
		x.l.Fatal("xray: cannot unmarshal Config file", zap.Error(err))
	}

	if err = newConfig.Validate(); err != nil {
		x.l.Fatal("xray: cannot validate Config file", zap.Error(err))
	}

	x.config = &newConfig
}

func (x *Xray) saveConfig() {
	content, err := json.Marshal(x.config)
	if err != nil {
		x.l.Fatal("xray: cannot marshal Config", zap.Error(err))
	}

	if err = os.WriteFile(x.configPath, content, 0755); err != nil {
		x.l.Fatal("xray: cannot save Config", zap.String("file", x.configPath), zap.Error(err))
	}
}

func (x *Xray) Run() {
	x.saveConfig()
	go x.runCore()
	x.connect()
}

func (x *Xray) RunWithConfig() {
	x.loadConfig()
	x.Run()
}

func (x *Xray) runCore() {
	if !utils.FileExist(x.binaryPath) {
		x.l.Fatal("xray: core binary file not found", zap.String("path", x.binaryPath))
	}

	x.command = exec.Command(x.binaryPath, "-c", x.configPath)
	x.command.Stderr = os.Stderr
	x.command.Stdout = os.Stdout

	x.l.Info("xray: starting the xray core...")
	if err := x.command.Run(); err != nil && err.Error() != "signal: killed" {
		x.l.Fatal("xray: cannot start the xray core", zap.Error(err))
	}
}

func (x *Xray) Restart() {
	x.l.Info("xray: restarting the xray core...")
	x.saveConfig()
	x.Shutdown()
	x.Run()
}

func (x *Xray) Shutdown() {
	if x.connection != nil {
		x.l.Info("xray: disconnecting xray core grpc...")
		_ = x.connection.Close()
	}
	if x.command != nil && x.command.Process != nil {
		x.l.Info("xray: shutting down the xray core...")
		if err := x.command.Process.Kill(); err != nil {
			x.l.Error("xray: failed to shutdown the xray core", zap.Error(err))
		} else {
			x.l.Info("xray: the xray core closed successfully")
		}
	}
}

func (x *Xray) connect() {
	inbound := x.config.FindInbound("api")
	if inbound == nil {
		x.l.Fatal("xray: cannot find api inbound")
	}

	address := "127.0.0.1:" + strconv.Itoa(inbound.Port)
	var err error
	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
		x.connection, err = grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			x.l.Debug("xray: trying to connect to grpc", zap.Int("try", i))
		} else {
			return
		}
	}

	x.l.Fatal("xray: cannot connect the xray core grpc", zap.Error(err))
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
		x.l.Error("xray: cannot fetch query stats", zap.Error(err))
	}
	return qs.GetStat()
}

func New(l *logger.Logger, configPath, binaryPath string) *Xray {
	return &Xray{l: l, config: NewConfig(), binaryPath: binaryPath, configPath: configPath}
}
