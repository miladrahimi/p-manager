package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	em "github.com/labstack/echo/v4/middleware"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/internal/http/handlers"
	"github.com/miladrahimi/p-manager/internal/http/handlers/api"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-node/pkg/http/client"
	cm "github.com/miladrahimi/p-node/pkg/http/middleware"
	"github.com/miladrahimi/p-node/pkg/http/validator"
	"github.com/miladrahimi/p-node/pkg/logger"
	"go.uber.org/zap"
)

// Server is the HTTP Server and holds all handler dependencies.
type Server struct {
	engine      *echo.Echo
	logger      *logger.Logger
	config      *config.Config
	coordinator *coordinator.Coordinator
	composer    *composer.Composer
	db          *data.Store
	hc          *client.Client
	sshClient   *ssh.Client
}

// New creates a new instance of HTTP Server.
func New(
	config *config.Config,
	logger *logger.Logger,
	composer *composer.Composer,
	coordinator *coordinator.Coordinator,
	db *data.Store,
	hc *client.Client,
	sshClient *ssh.Client,
) *Server {
	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()
	return &Server{
		engine:      e,
		logger:      logger,
		config:      config,
		coordinator: coordinator,
		composer:    composer,
		db:          db,
		hc:          hc,
		sshClient:   sshClient,
	}
}

// Run defines the required HTTP routes and starts the HTTP Server.
func (s *Server) Run() {
	s.engine.Use(cm.Logger(s.logger))
	s.engine.Use(cm.General())
	s.engine.Use(em.CORS())

	// Serve static admin panel UI
	s.engine.Static("/", "web")

	// Pages: Public
	s.engine.GET("/account/:accountId", handlers.Account(s.db))
	s.engine.GET("/subscription/:proxyId", api.SubscriptionShow(s.composer, s.db))

	// APIs: Public
	g1 := s.engine.Group("api")
	g1.POST("/sign-in", api.SignIn(s.db))
	g1.GET("/account/:accountId", api.AccountShow(s.composer, s.db))
	g1.POST("/account/:accountId/links/renew", api.AccountLinksRenew(s.coordinator, s.db))

	// APIs: Admin
	g2 := s.engine.Group("api")
	g2.Use(cm.Authorize(func() string {
		var token string
		s.db.Read(func(d *data.Data) {
			token = d.MainSettings.AdminPassword
		})
		return token
	}))

	g2.GET("/accounts", api.AccountsIndex(s.db))
	g2.POST("/accounts", api.AccountsStore(s.coordinator, s.db))
	g2.PATCH("/accounts", api.AccountsUpdatePartialBatch(s.coordinator, s.db))
	g2.PUT("/accounts/:id", api.AccountsUpdate(s.coordinator, s.db))
	g2.PATCH("/accounts/:id", api.AccountsUpdatePartial(s.coordinator, s.db))
	g2.DELETE("/accounts/:id", api.AccountsDelete(s.coordinator, s.db))
	g2.DELETE("/accounts", api.AccountsDeleteBatch(s.coordinator, s.db))
	g2.POST("/accounts/import", api.AccountsImport(s.coordinator, s.db, s.hc))

	g2.GET("/nodes", api.NodesIndex(s.db))
	g2.POST("/nodes", api.NodesStore(s.coordinator, s.db))
	g2.PATCH("/nodes", api.NodesUpdatePartialBatch(s.coordinator, s.db))
	g2.PUT("/nodes/:id", api.NodesUpdate(s.coordinator, s.db))
	g2.DELETE("/nodes/:id", api.NodesDelete(s.coordinator, s.db))
	g2.GET("/nodes/:id/config", api.NodesConfigShow(s.coordinator, s.composer, s.db))

	g2.GET("/stats", api.StatsIndex(s.db))
	g2.PATCH("/stats", api.StatsUpdatePartial(s.db))

	g2.GET("/platform", api.PlatformShow())

	g2.GET("/insights", api.InsightsIndex(s.db))

	g2.GET("/main-settings", api.MainSettingsShow(s.db))
	g2.POST("/main-settings", api.MainSettingsUpdate(s.db))

	g2.POST("/xray/restart", api.XrayRestart(s.coordinator))

	g2.GET("/xray-settings", api.XraySettingsShow(s.db))
	g2.POST("/xray-settings", api.XraySettingsUpdate(s.coordinator, s.db))

	// Start the HTTP Server
	go func() {
		address := fmt.Sprintf("%s:%d", s.config.HttpServer.Host, s.config.HttpServer.Port)
		if err := s.engine.Start(address); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Fatal(
				"http server: cannot start",
				zap.String("address", address),
				zap.Error(errors.WithStack(err)),
			)
		}
	}()
}

// Close closes the HTTP Server.
func (s *Server) Close() error {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.engine.Shutdown(c); err != nil {
		return errors.WithStack(err)
	}

	s.logger.Debug("http server: closed successfully")
	return nil
}
