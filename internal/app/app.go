package app

import (
	"context"
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/xray-manager/internal/config"
	"github.com/miladrahimi/xray-manager/internal/coordinator"
	"github.com/miladrahimi/xray-manager/internal/database"
	"github.com/miladrahimi/xray-manager/internal/http/server"
	"github.com/miladrahimi/xray-manager/pkg/enigma"
	"github.com/miladrahimi/xray-manager/pkg/fetcher"
	"github.com/miladrahimi/xray-manager/pkg/logger"
	"github.com/miladrahimi/xray-manager/pkg/xray"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

type App struct {
	context     context.Context
	Config      *config.Config
	Log         *logger.Logger
	Fetcher     *fetcher.Fetcher
	HttpServer  *server.Server
	Database    *database.Database
	Coordinator *coordinator.Coordinator
	Xray        *xray.Xray
	Enigma      *enigma.Enigma
}

func New() (a *App, err error) {
	a = &App{}

	a.Config = config.New()
	if err = a.Config.Init(); err != nil {
		return nil, errors.WithStack(err)
	}
	a.Log = logger.New(a.Config.Logger.Level, a.Config.Logger.Format, a.ShutdownModules)
	if err = a.Log.Init(); err != nil {
		return nil, errors.WithStack(err)
	}

	a.Log.Info("app: logger and Config initialized successfully")

	a.Database = database.New(a.Log)
	a.Xray = xray.New(a.Log, config.XrayConfigPath, a.Config.XrayBinaryPath())
	a.Enigma = enigma.New(config.EnigmaKeyPath)
	a.Fetcher = fetcher.New(a.Config.HttpClient.Timeout)
	a.Coordinator = coordinator.New(a.Config, a.Fetcher, a.Log, a.Database, a.Xray, a.Enigma)
	a.HttpServer = server.New(a.Config, a.Log, a.Coordinator, a.Database, a.Enigma)

	a.Log.Info("app: modules initialized successfully")

	a.setupSignalListener()

	return a, nil
}

func (a *App) Init() error {
	err := a.Database.Init()
	return errors.WithStack(err)
}

func (a *App) setupSignalListener() {
	var cancel context.CancelFunc
	a.context, cancel = context.WithCancel(context.Background())

	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
		s := <-signalChannel
		a.Log.Info("app: system call", zap.String("signal", s.String()))
		cancel()
	}()
}

func (a *App) Wait() {
	<-a.context.Done()
}

func (a *App) ShutdownModules() {
	a.Log.Info("app: shutting down modules...")
	if a.HttpServer != nil {
		a.HttpServer.Shutdown()
	}
	if a.Xray != nil {
		a.Xray.Shutdown()
	}
}

func (a *App) Shutdown() {
	a.Log.Info("app: shutting down...")
	a.ShutdownModules()
	if a.Log != nil {
		a.Log.Shutdown()
	}
}
