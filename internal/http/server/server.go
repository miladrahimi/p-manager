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
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/http/client"
	cm "github.com/miladrahimi/p-node/pkg/http/middleware"
	"github.com/miladrahimi/p-node/pkg/http/validator"
	"github.com/miladrahimi/p-node/pkg/logger"
	"go.uber.org/zap"
)

type Server struct {
	engine      *echo.Echo
	logger      *logger.Logger
	config      *config.Config
	coordinator *coordinator.Coordinator
	composer    *composer.Composer
	db          *database.Database[data.Data]
	hc          *client.Client
}

// New creates a new instance of HTTP Server.
func New(
	config *config.Config,
	logger *logger.Logger,
	composer *composer.Composer,
	coordinator *coordinator.Coordinator,
	db *database.Database[data.Data],
	hc *client.Client,
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
	}
}

// Run defines the required HTTP routes and starts the HTTP Server.
func (s *Server) Run() {
	s.engine.Use(cm.Logger(s.logger))
	s.engine.Use(cm.General())
	s.engine.Use(em.CORS())

	// Serve static admin panel UI
	s.engine.Static("/", "web")
	s.engine.GET("/profile", handlers.Profile(s.db))

	// APIs: Guest
	g1 := s.engine.Group("api")
	g1.POST("/sign-in", api.SignIn(s.db))
	g1.GET("/profile", api.ProfileShow(s.composer, s.db))
	g1.POST("/profile/links/renew", api.ProfileLinksRenew(s.coordinator, s.db))

	// APIs: Admin
	g2 := s.engine.Group("api")
	g2.Use(cm.Authorize(func() string {
		return s.db.Data().MainSettings.AdminPassword
	}))

	g2.GET("/users", api.UsersIndex(s.db))
	g2.POST("/users", api.UsersStore(s.coordinator, s.db))
	g2.PATCH("/users", api.UsersUpdatePartialBatch(s.coordinator, s.db))
	g2.PUT("/users/:id", api.UsersUpdate(s.coordinator, s.db))
	g2.PATCH("/users/:id", api.UsersUpdatePartial(s.coordinator, s.db))
	g2.DELETE("/users/:id", api.UsersDelete(s.coordinator, s.db))
	g2.DELETE("/users", api.UsersDeleteBatch(s.coordinator, s.db))
	g2.POST("/users/import", api.UsersImport(s.coordinator, s.db, s.hc))

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
