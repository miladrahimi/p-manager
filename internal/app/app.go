package app

import (
	"context"
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/enigma"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/internal/http/server"
	"github.com/miladrahimi/p-manager/internal/licensor"
	"github.com/miladrahimi/p-manager/internal/writer"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

type App struct {
	Context     context.Context
	Cancel      context.CancelFunc
	Shutdown    chan struct{}
	Config      *config.Config
	Logger      *logger.Logger
	HttpClient  *client.Client
	HttpServer  *server.Server
	Database    *database.Database
	Writer      *writer.Writer
	Coordinator *coordinator.Coordinator
	Xray        *xray.Xray
	Enigma      *enigma.Enigma
	Licensor    *licensor.Licensor
}

func New() (a *App, err error) {
	a = &App{}
	a.Context, a.Cancel = context.WithCancel(context.Background())
	a.Shutdown = make(chan struct{})

	wd, err := os.Getwd()
	if err != nil {
		return a, errors.WithStack(err)
	}

	a.Config = config.New(config.NewEnv(wd))
	if err = a.Config.Init(); err != nil {
		return a, errors.WithStack(err)
	}
	a.Logger = logger.New(a.Config.Logger.Level, a.Config.Logger.Format, a.Shutdown)
	if err = a.Logger.Init(); err != nil {
		return a, errors.WithStack(err)
	}

	c := a.Config
	e := a.Config.Env

	a.Database = database.New(a.Logger, c)
	a.Xray = xray.New(a.Context, a.Logger, c.Xray.LogLevel, e.XrayConfigPath, e.XrayBinaryPath)
	a.HttpClient = client.New(c.HttpClient.Timeout, config.AppName, config.AppVersion)
	a.Enigma = enigma.New(e.EnigmaKeyPath)
	a.Licensor = licensor.New(c, a.HttpClient, a.Logger, a.Database, a.Enigma)
	a.Writer = writer.New(a.Config, a.Database, a.Xray)
	a.Coordinator = coordinator.New(c, a.Context, a.HttpClient, a.Logger, a.Database, a.Xray, a.Writer)
	a.HttpServer = server.New(c, a.Logger, a.Coordinator, a.Database, a.Enigma, a.Licensor, a.Writer, a.HttpClient)

	a.Logger.Info("app: constructed successfully")

	a.setupSignalListener()

	return a, nil
}

func (a *App) Start() error {
	if err := a.Database.Init(); err != nil {
		return errors.WithStack(err)
	}
	if err := a.Enigma.Init(); err != nil {
		return errors.WithStack(err)
	}

	a.Licensor.Run()
	a.Coordinator.Run()
	a.HttpServer.Run()

	a.Logger.Info("app: initialized successfully")
	return nil
}

func (a *App) setupSignalListener() {
	go func() {
		signalChannel := make(chan os.Signal, 2)
		signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
		s := <-signalChannel
		a.Logger.Info("app: signal received", zap.String("signal", s.String()))
		a.Cancel()
	}()

	go func() {
		<-a.Shutdown
		a.Cancel()
	}()
}

func (a *App) Wait() {
	a.Logger.Debug("app: waiting...")
	<-a.Context.Done()
}

func (a *App) Close() {
	a.Logger.Debug("app: closing...")
	defer a.Logger.Info("app: closed")

	if a.HttpServer != nil {
		a.HttpServer.Close()
	}
	if a.Xray != nil {
		if err := a.Xray.Close(); err != nil {
			a.Logger.Error("xray: cannot close", zap.Error(errors.WithStack(err)))
		}
	}
	if a.Database != nil {
		a.Database.Close()
	}
	if a.Logger != nil {
		a.Logger.Close()
	}
}
