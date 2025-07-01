package server

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/coordinator"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/enigma"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/internal/http/handlers/pages"
	"github.com/miladrahimi/p-manager/internal/http/handlers/v1"
	"github.com/miladrahimi/p-manager/internal/licensor"
	"github.com/miladrahimi/p-manager/internal/writer"
	"github.com/miladrahimi/p-node/pkg/http/middleware"
	"github.com/miladrahimi/p-node/pkg/http/validator"
	"github.com/miladrahimi/p-node/pkg/logger"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type Server struct {
	e           *echo.Echo
	l           *logger.Logger
	config      *config.Config
	coordinator *coordinator.Coordinator
	database    *database.Database
	enigma      *enigma.Enigma
	licensor    *licensor.Licensor
	writer      *writer.Writer
	hc          *client.Client
}

// Run defines the required HTTP routes and starts the HTTP Server.
func (s *Server) Run() {
	s.e.Use(middleware.Logger(s.l))
	s.e.Use(middleware.General())
	s.e.Use(echoMiddleware.CORS())

	s.e.Static("/", "web")
	s.e.GET("/profile", pages.Profile(s.config, s.database))

	g1 := s.e.Group("/v1")
	g1.POST("/sign-in", v1.SignIn(s.database, s.enigma))

	g1.GET("/profile", v1.ProfileShow(s.database))
	g1.POST("/profile/links/regenerate", v1.ProfileRegenerate(s.coordinator, s.database))

	g2 := s.e.Group("/v1")
	g2.Use(middleware.Authorize(func() string {
		return s.database.Content.Settings.AdminPassword
	}))

	g2.GET("/users", v1.UsersIndex(s.database))
	g2.POST("/users", v1.UsersStore(s.coordinator, s.database, s.licensor))
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

	g2.GET("/nodes/:id/configs", v1.NodesConfigsShow(s.coordinator, s.writer, s.database))

	g2.GET("/stats", v1.StatsIndex(s.database))
	g2.PATCH("/stats", v1.StatsUpdatePartial(s.database))

	g2.GET("/information", v1.InformationIndex(s.licensor))

	g2.GET("/settings", v1.SettingsShow(s.database))
	g2.POST("/settings", v1.SettingsUpdate(s.coordinator, s.database))
	g2.POST("/settings/xray/restart", v1.SettingsXrayRestart(s.coordinator))

	g2.POST("/imports", v1.ImportsStore(s.database, s.hc))

	go func() {
		address := fmt.Sprintf("%s:%d", s.config.HttpServer.Host, s.config.HttpServer.Port)
		if err := s.e.Start(address); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.l.Fatal(
				"http server:  cannot start",
				zap.String("address", address),
				zap.Error(errors.WithStack(err)),
			)
		}
	}()
}

// Close closes the HTTP Server.
func (s *Server) Close() {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.e.Shutdown(c); err != nil {
		s.l.Error("http server:  cannot close", zap.Error(errors.WithStack(err)))
	} else {
		s.l.Info("http server:  closed successfully")
	}
}

// New creates a new instance of HTTP Server.
func New(
	config *config.Config,
	logger *logger.Logger,
	c *coordinator.Coordinator,
	database *database.Database,
	enigma *enigma.Enigma,
	licensor *licensor.Licensor,
	writer *writer.Writer,
	hc *client.Client,
) *Server {
	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()
	return &Server{
		e:           e,
		l:           logger,
		config:      config,
		coordinator: c,
		database:    database,
		enigma:      enigma,
		licensor:    licensor,
		writer:      writer,
		hc:          hc,
	}
}
