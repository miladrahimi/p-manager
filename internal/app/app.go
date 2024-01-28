package app

import (
	"context"
	"github.com/miladrahimi/xray-manager/internal/config"
	"github.com/miladrahimi/xray-manager/internal/coordinator"
	"github.com/miladrahimi/xray-manager/internal/database"
	"github.com/miladrahimi/xray-manager/internal/http/server"
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
	config      *config.Config
	log         *logger.Logger
	fetcher     *fetcher.Fetcher
	httpServer  *server.Server
	database    *database.Database
	coordinator *coordinator.Coordinator
	xray        *xray.Xray
}

func New() (a *App, err error) {
	a = &App{}

	a.config = config.New()
	if err = a.config.Init(); err != nil {
		return nil, err
	}
	a.log = logger.New(a.config.Logger.Level, a.config.Logger.Format, a.ShutdownModules)
	if err = a.log.Init(); err != nil {
		return nil, err
	}

	a.log.Info("app: logger and config initialized")

	a.database = database.New(a.log)
	a.xray = xray.New(a.log, a.config.XrayConfigPath(), a.config.XrayBinaryPath())
	a.fetcher = fetcher.New(a.config.HttpClient.Timeout)
	a.coordinator = coordinator.New(a.config, a.fetcher, a.log, a.database, a.xray)
	a.httpServer = server.New(a.config, a.log, a.coordinator, a.database)

	a.setupSignalListener()

	return a, nil
}

func (a *App) Boot() {
	a.database.Init()
	a.xray.Run()
	a.coordinator.Run()
	a.httpServer.Run()
}

func (a *App) setupSignalListener() {
	var cancel context.CancelFunc
	a.context, cancel = context.WithCancel(context.Background())

	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

		s := <-signalChannel
		a.log.Info("app: system call", zap.String("signal", s.String()))

		cancel()
	}()

	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, syscall.SIGHUP)

		for {
			s := <-signalChannel
			a.log.Info("app: system call", zap.String("signal", s.String()))
			a.xray.Restart()
		}
	}()
}

func (a *App) Wait() {
	<-a.context.Done()
}

func (a *App) ShutdownModules() {
	a.log.Info("app: shutting down modules...")
	if a.httpServer != nil {
		a.httpServer.Shutdown()
	}
	if a.xray != nil {
		a.xray.Shutdown()
	}
}

func (a *App) Shutdown() {
	a.log.Info("app: shutting down...")
	a.ShutdownModules()
	if a.log != nil {
		a.log.Shutdown()
	}
}
