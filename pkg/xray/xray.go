package xray

import (
	"context"
	"encoding/json"
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/pkg/logger"
	"github.com/miladrahimi/p-manager/pkg/utils"
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
	l          *logger.Logger
	config     *Config
	configPath string
	binaryPath string
	command    *exec.Cmd
	connection *grpc.ClientConn
	locker     *sync.Mutex
	context    context.Context
}

func (x *Xray) loadConfig() {
	x.l.Debug("xray: loading config file...")

	if !utils.FileExist(x.configPath) {
		x.l.Debug("xray: no config file to load")
		return
	}

	defer x.l.Debug("xray: config file loaded")

	content, err := os.ReadFile(x.configPath)
	if err != nil {
		x.l.Fatal("xray: cannot load config file", zap.Error(errors.WithStack(err)))
	}

	var newConfig Config
	err = json.Unmarshal(content, &newConfig)
	if err != nil {
		x.l.Fatal("xray: cannot unmarshal load config file", zap.Error(errors.WithStack(err)))
	}

	if err = newConfig.Validate(); err != nil {
		x.l.Fatal("xray: cannot validate load config file", zap.Error(errors.WithStack(err)))
	}

	x.config = &newConfig
}

func (x *Xray) saveConfig() {
	x.l.Debug("xray: saving config...")
	defer x.l.Debug("xray: config file saved")

	content, err := json.Marshal(x.config)
	if err != nil {
		x.l.Fatal("xray: cannot marshal config data", zap.Error(errors.WithStack(err)))
	}

	if err = os.WriteFile(x.configPath, content, 0755); err != nil {
		x.l.Fatal("xray: cannot save config data", zap.Error(errors.WithStack(err)))
	}
}

func (x *Xray) run() {
	x.l.Debug("xray: running...")
	go x.runCore()
	x.connect()
}

func (x *Xray) SaveConfigAndRun() {
	x.l.Debug("xray: saving config and running...")

	x.locker.Lock()
	defer x.locker.Unlock()

	x.saveConfig()
	x.run()
}

func (x *Xray) LoadConfigAndRun() {
	x.l.Debug("xray: loading config and running...")

	x.locker.Lock()
	defer x.locker.Unlock()

	x.loadConfig()
	x.run()
}

func (x *Xray) runCore() {
	x.l.Debug("xray: running core...")

	if !utils.FileExist(x.binaryPath) {
		x.l.Fatal("xray: core binary file not found", zap.String("path", x.binaryPath))
	}

	x.command = exec.Command(x.binaryPath, "-c", x.configPath)
	x.command.Stderr = os.Stderr
	x.command.Stdout = os.Stdout

	x.l.Info("xray: running xray core binary...", zap.String("path", x.binaryPath))
	if err := x.command.Run(); err != nil && err.Error() != "signal: killed" {
		x.l.Fatal("xray: cannot start the xray core", zap.Error(errors.WithStack(err)))
	}
}

func (x *Xray) Restart() {
	x.l.Info("xray: restarting...")
	x.Close()
	x.SaveConfigAndRun()
}

func (x *Xray) Close() {
	x.l.Info("xray: shutting down...")

	x.locker.Lock()
	defer x.locker.Unlock()

	if x.connection != nil {
		x.l.Info("xray: disconnecting xray core grpc...")
		_ = x.connection.Close()
	}
	if x.command != nil && x.command.Process != nil {
		x.l.Info("xray: stopping down the xray core...")
		if err := x.command.Process.Kill(); err != nil {
			x.l.Error("xray: cannot stop the xray core", zap.Error(errors.WithStack(err)))
		} else {
			x.l.Info("xray: the xray core closed successfully")
		}
	}
}

func (x *Xray) connect() {
	x.l.Debug("xray: connecting to xray core api...")

	inbound := x.config.FindInbound("api")
	if inbound == nil {
		x.l.Fatal("xray: cannot find api inbound")
	}

	c, cancel := context.WithTimeout(x.context, 10*time.Second)
	defer cancel()

	address := "127.0.0.1:" + strconv.Itoa(inbound.Port)
	var err error

	for {
		select {
		case <-c.Done():
			x.l.Fatal("xray: cannot connect to grpc api", zap.Error(errors.WithStack(x.context.Err())))
			return
		default:
			time.Sleep(time.Second)
			x.connection, err = grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				x.l.Debug("xray: trying to connect to grpc")
			} else {
				x.l.Debug("xray: connected to api successfully")
				return
			}
		}
	}
}

func (x *Xray) Config() *Config {
	return x.config
}

func (x *Xray) SetConfig(config *Config) {
	x.config = config
}

func (x *Xray) QueryStats() []*stats.Stat {
	client := stats.NewStatsServiceClient(x.connection)
	qs, err := client.QueryStats(context.Background(), &stats.QueryStatsRequest{Reset_: true})
	if err != nil {
		x.l.Error("xray: cannot fetch query stats", zap.Error(errors.WithStack(err)))
	}
	return qs.GetStat()
}

func New(c context.Context, logger *logger.Logger, configPath, binaryPath string) *Xray {
	return &Xray{
		context:    c,
		l:          logger,
		config:     NewConfig(),
		binaryPath: binaryPath,
		configPath: configPath,
		locker:     &sync.Mutex{},
	}
}
