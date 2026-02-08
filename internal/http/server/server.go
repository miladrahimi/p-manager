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
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/internal/http/handlers"
	"github.com/miladrahimi/p-manager/internal/http/handlers/v1"
	"github.com/miladrahimi/p-node/pkg/database"
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
	database    *database.Database[data.Data]
	composer    *composer.Composer
	hc          *client.Client
}

// New creates a new instance of HTTP Server.
func New(
	config *config.Config,
	logger *logger.Logger,
	c *coordinator.Coordinator,
	database *database.Database[data.Data],
	writer *composer.Composer,
	hc *client.Client,
) *Server {
	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()
	return &Server{
		engine:      e,
		logger:      logger,
		config:      config,
		coordinator: c,
		database:    database,
		composer:    writer,
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

	// Root APIs
	s.engine.GET("/profile", handlers.Profile(s.database))

	// V1 APIs: Guest
	g1 := s.engine.Group("/v1")
	g1.POST("/sign-in", v1.SignIn(s.database))
	g1.GET("/profile", v1.ProfileShow(s.database))
	g1.POST("/profile/links/regenerate", v1.ProfileRegenerate(s.coordinator, s.database))

	// V1 APIs: Admin
	g2 := s.engine.Group("/v1")
	g2.Use(cm.Authorize(func() string {
		return s.database.Data().Settings.AdminPassword
	}))

	g2.GET("/users", v1.UsersIndex(s.database))
	g2.POST("/users", v1.UsersStore(s.coordinator, s.database))
	g2.PATCH("/users", v1.UsersUpdatePartialBatch(s.coordinator, s.database))
	g2.PUT("/users/:id", v1.UsersUpdate(s.coordinator, s.database))
	g2.PATCH("/users/:id", v1.UsersUpdatePartial(s.coordinator, s.database))
	g2.DELETE("/users/:id", v1.UsersDelete(s.coordinator, s.database))
	g2.DELETE("/users", v1.UsersDeleteBatch(s.coordinator, s.database))

	g2.GET("/nodes", v1.NodesIndex(s.database))
	g2.POST("/nodes", v1.NodesStore(s.coordinator, s.database))
	g2.PATCH("/nodes", v1.NodesUpdatePartialBatch(s.coordinator, s.database))
	g2.PUT("/nodes/:id", v1.NodesUpdate(s.coordinator, s.database))
	g2.DELETE("/nodes/:id", v1.NodesDelete(s.coordinator, s.database))

	g2.GET("/nodes/:id/configs", v1.NodesConfigsShow(s.coordinator, s.composer, s.database))

	g2.GET("/stats", v1.StatsIndex(s.database))
	g2.PATCH("/stats", v1.StatsUpdatePartial(s.database))

	g2.GET("/information", v1.InformationIndex())

	g2.GET("/settings", v1.SettingsShow(s.database))
	g2.POST("/settings", v1.SettingsUpdate(s.coordinator, s.database))
	g2.POST("/settings/xray/restart", v1.SettingsXrayRestart(s.coordinator))

	g2.POST("/imports", v1.ImportsStore(s.coordinator, s.database, s.hc))

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
