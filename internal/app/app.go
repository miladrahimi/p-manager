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
	shutdown    chan struct{}
	Config      *config.Config
	Logger      *logger.Logger
	Fetcher     *fetcher.Fetcher
	HttpServer  *server.Server
	Database    *database.Database
	Coordinator *coordinator.Coordinator
	Xray        *xray.Xray
	Enigma      *enigma.Enigma
}

func New() (a *App, err error) {
	a = &App{}
	a.shutdown = make(chan struct{})

	a.Config = config.New()
	if err = a.Config.Init(); err != nil {
		return nil, errors.WithStack(err)
	}
	a.Logger = logger.New(a.Config.Logger.Level, a.Config.Logger.Format, a.shutdown)
	if err = a.Logger.Init(); err != nil {
		return nil, errors.WithStack(err)
	}

	a.Logger.Info("app: logger and Config initialized successfully")

	a.Database = database.New(a.Logger)
	a.Xray = xray.New(a.Logger, config.XrayConfigPath, a.Config.XrayBinaryPath())
	a.Enigma = enigma.New(config.EnigmaKeyPath)
	a.Fetcher = fetcher.New(a.Config.HttpClient.Timeout)
	a.Coordinator = coordinator.New(a.Config, a.Fetcher, a.Logger, a.Database, a.Xray, a.Enigma)
	a.HttpServer = server.New(a.Config, a.Logger, a.Coordinator, a.Database, a.Enigma)

	a.Logger.Info("app: modules initialized successfully")

	a.setupSignalListener()

	return a, nil
}

func (a *App) Init() {
	a.Database.Init()
	a.Logger.Info("app: initialized successfully")
}

func (a *App) setupSignalListener() {
	var cancel context.CancelFunc
	a.context, cancel = context.WithCancel(context.Background())

	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
		s := <-signalChannel
		a.Logger.Info("app: system call", zap.String("signal", s.String()))
		cancel()
	}()

	go func() {
		<-a.shutdown
		cancel()
	}()
}

func (a *App) Wait() {
	<-a.context.Done()
}

func (a *App) Shutdown() {
	a.Logger.Info("app: shutting down...")
	if a.HttpServer != nil {
		a.HttpServer.Shutdown()
	}
	if a.Xray != nil {
		a.Xray.Shutdown()
	}
	if a.Logger != nil {
		a.Logger.Shutdown()
	}
}
