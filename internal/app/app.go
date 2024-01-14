package app

import (
	"context"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"shadowsocks-manager/internal/config"
	"shadowsocks-manager/internal/coordinator"
	"shadowsocks-manager/internal/database"
	"shadowsocks-manager/internal/http/client"
	"shadowsocks-manager/internal/http/server"
	"shadowsocks-manager/internal/logger"
	"shadowsocks-manager/internal/xray"
	"syscall"
)

// App integrates the modules to serve.
type App struct {
	context     context.Context
	config      *config.Config
	log         *logger.Logger
	httpClient  *http.Client
	httpServer  *server.Server
	database    *database.Database
	coordinator *coordinator.Coordinator
	xray        *xray.Xray
}

// New creates an instance of the application with dependencies injected.
func New() (app *App, err error) {
	app = &App{}

	app.config = config.New()
	if app.config.Init() != nil {
		return nil, err
	}
	app.log = logger.New(app.config)
	if app.log.Init() != nil {
		return nil, err
	}

	app.database = database.New(app.log.Engine)
	app.xray = xray.New(app.log.Engine)
	app.httpClient = client.New(app.config)
	app.coordinator = coordinator.New(app.config, app.httpClient, app.log.Engine, app.database, app.xray)
	app.httpServer = server.New(app.config, app.log.Engine, app.coordinator, app.database)

	app.setupSignalListener()

	return app, nil
}

// Boot initializes application modules
func (a *App) Boot() {
	a.database.Init()
	a.xray.Run()
	a.coordinator.Run()
	a.httpServer.Run()
}

// setupSignalListener sets up a listener to signals from os.
func (a *App) setupSignalListener() {
	var cancel context.CancelFunc
	a.context, cancel = context.WithCancel(context.Background())

	// Listen to SIGTERM
	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

		s := <-signalChannel
		a.log.Engine.Info("app: system call", zap.String("signal", s.String()))

		cancel()
	}()

	// Listen to SIGHUP
	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, syscall.SIGHUP)

		for {
			s := <-signalChannel
			a.log.Engine.Info("app: system call", zap.String("signal", s.String()))
			a.xray.Reconfigure()
		}
	}()
}

// Wait avoid dying app and shut it down gracefully on exit signals.
func (a *App) Wait() {
	<-a.context.Done()
}

// Shutdown closes all open resources and processes gracefully.
func (a *App) Shutdown() {
	if a.httpServer != nil {
		a.httpServer.Shutdown()
	}
	if a.xray != nil {
		a.xray.Shutdown()
	}
	if a.log != nil {
		a.log.Shutdown()
	}
}
