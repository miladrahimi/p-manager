package app

import (
	"context"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"shadowsocks-manager/internal/config"
	"shadowsocks-manager/internal/coordinator"
	"shadowsocks-manager/internal/database"
	"shadowsocks-manager/internal/http/server"
	"shadowsocks-manager/internal/logger"
	"shadowsocks-manager/internal/xray"
	"syscall"
)

// App integrates the modules to serve.
type App struct {
	Context     context.Context
	Config      *config.Config
	Logger      *logger.Logger
	HttpServer  *server.Server
	Database    *database.Database
	Coordinator *coordinator.Coordinator
	Xray        *xray.Xray
}

// New creates an instance of the application with dependencies injected.
func New() (app *App, err error) {
	app = &App{}

	app.Config = config.New()
	if app.Config.Init() != nil {
		return nil, err
	}
	app.Logger = logger.New(app.Config)
	if app.Logger.Init() != nil {
		return nil, err
	}

	app.Database = database.New(app.Logger.Engine)
	app.Xray = xray.New(app.Logger.Engine)
	app.Coordinator = coordinator.New(app.Config, app.Logger.Engine, app.Database, app.Xray)
	app.HttpServer = server.New(app.Config, app.Logger.Engine, app.Coordinator, app.Database)

	app.setupSignalListener()

	return app, nil
}

// Boot initializes application modules
func (a *App) Boot() {
	a.Database.Init()
	a.Xray.Run()
	a.Coordinator.Run()
	a.HttpServer.Run()
}

// setupSignalListener sets up a listener to signals from os.
func (a *App) setupSignalListener() {
	var cancel context.CancelFunc
	a.Context, cancel = context.WithCancel(context.Background())

	// Listen to SIGTERM
	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

		s := <-signalChannel
		a.Logger.Engine.Info("app: system call", zap.String("signal", s.String()))

		cancel()
	}()

	// Listen to SIGHUP
	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, syscall.SIGHUP)

		for {
			s := <-signalChannel
			a.Logger.Engine.Info("app: system call", zap.String("signal", s.String()))
			a.Xray.Reconfigure()
		}
	}()
}

// Wait avoid dying app and shut it down gracefully on exit signals.
func (a *App) Wait() {
	<-a.Context.Done()
}

// Shutdown closes all open resources and processes gracefully.
func (a *App) Shutdown() {
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
