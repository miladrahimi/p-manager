package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/internal/http/server"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
	"go.uber.org/zap"
)

// App represents the application.
type App struct {
	context     context.Context
	cancel      context.CancelFunc
	shutdown    chan struct{}
	config      *config.Config
	logger      *logger.Logger
	httpClient  *client.Client
	httpServer  *server.Server
	database    *database.Database[data.Data]
	composer    *composer.Composer
	coordinator *coordinator.Coordinator
	xray        *xray.Xray
}

// New creates a new instance of the App.
func New() (a *App, err error) {
	a = &App{}
	a.context, a.cancel = context.WithCancel(context.Background())
	a.shutdown = make(chan struct{})

	if a.config, err = config.New(); err != nil {
		return a, errors.WithStack(err)
	}

	a.logger, err = logger.New(a.config.Logger.Level, a.config.Logger.Format, a.shutdown)
	if err != nil {
		return a, errors.WithStack(err)
	}

	c := a.config

	a.httpClient = client.New(c.HttpClient.Timeout, config.AppName, config.AppVersion)
	if a.database, err = database.New(config.DatabaseFilePath, data.Default()); err != nil {
		return a, errors.WithStack(err)
	}
	a.xray = xray.New(a.context, a.logger, c.Xray.LogLevel, config.XrayConfigPath, config.XrayBinaryPath())
	a.composer = composer.New(a.config, a.database, a.xray)
	a.coordinator = coordinator.New(c, a.httpClient, a.logger, a.database, a.xray, a.composer, config.DatabaseBackupPath)
	a.httpServer = server.New(c, a.logger, a.coordinator, a.database, a.composer, a.httpClient)

	a.logger.Info("app: constructed successfully")

	a.setupSignalListener()

	return a, nil
}

// Start starts the app and its components.
func (a *App) Start() error {
	if err := a.database.Init(); err != nil {
		return errors.WithStack(err)
	}

	a.coordinator.Run(a.context)
	a.httpServer.Run()

	a.logger.Info("app: initialized successfully")
	return nil
}

// setupSignalListener sets up a signal listener to handle interrupt and termination signals.
func (a *App) setupSignalListener() {
	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
		s := <-signalChannel
		a.logger.Info("app: signal received", zap.String("signal", s.String()))
		a.cancel()
	}()

	go func() {
		<-a.shutdown
		a.cancel()
	}()
}

// Wait waits for the app context to be canceled.
func (a *App) Wait() {
	a.logger.Debug("app: waiting...")
	<-a.context.Done()
}

// Close closes the app and its components.
func (a *App) Close() {
	a.logger.Debug("app: closing...")
	defer a.logger.Info("app: closed")

	if a.httpServer != nil {
		if err := a.httpServer.Close(); err != nil {
			a.logger.Error("cannot close http server", zap.Error(errors.WithStack(err)))
		}
	}
	if a.xray != nil {
		if err := a.xray.Stop(); err != nil {
			a.logger.Error("cannot close xray", zap.Error(errors.WithStack(err)))
		}
	}
	if a.database != nil {
		if err := a.database.Save(); err != nil {
			a.logger.Error("cannot save database", zap.Error(errors.WithStack(err)))
		}
	}
	if a.logger != nil {
		a.logger.Close()
	}
}
