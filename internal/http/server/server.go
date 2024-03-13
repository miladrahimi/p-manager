package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/miladrahimi/xray-manager/internal/config"
	"github.com/miladrahimi/xray-manager/internal/coordinator"
	"github.com/miladrahimi/xray-manager/internal/database"
	"github.com/miladrahimi/xray-manager/internal/http/handlers/pages"
	"github.com/miladrahimi/xray-manager/internal/http/handlers/v1"
	"github.com/miladrahimi/xray-manager/pkg/enigma"
	"github.com/miladrahimi/xray-manager/pkg/logger"
	mw "github.com/miladrahimi/xray-manager/pkg/routing/middleware"
	"github.com/miladrahimi/xray-manager/pkg/routing/validator"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type Server struct {
	engine      *echo.Echo
	config      *config.Config
	l           *logger.Logger
	coordinator *coordinator.Coordinator
	database    *database.Database
	enigma      *enigma.Enigma
}

// Run defines the required HTTP routes and starts the HTTP Server.
func (s *Server) Run() {
	s.engine.Use(mw.Logger(s.l))
	s.engine.Use(echoMiddleware.CORS())

	s.engine.Static("/", "web")
	s.engine.GET("/profile", pages.Profile())

	g1 := s.engine.Group("/v1")
	g1.POST("/sign-in", v1.SignIn(s.database, s.enigma))

	g1.GET("/profile", v1.ProfileShow(s.database))
	g1.POST("/profile/reset", v1.ProfileReset(s.coordinator, s.database))

	g2 := s.engine.Group("/v1")
	g2.Use(mw.Authorize(func() string {
		return s.database.Data.Settings.AdminPassword
	}))

	g2.GET("/users", v1.UsersIndex(s.database))
	g2.POST("/users", v1.UsersStore(s.coordinator, s.database))
	g2.PUT("/users", v1.UsersUpdate(s.coordinator, s.database))
	g2.DELETE("/users/:id", v1.UsersDelete(s.coordinator, s.database))
	g2.PATCH("/users/:id/zero", v1.UsersZero(s.coordinator, s.database))

	g2.GET("/servers", v1.ServersIndex(s.database))
	g2.POST("/servers", v1.ServersStore(s.coordinator, s.database))
	g2.PUT("/servers", v1.ServersUpdate(s.coordinator, s.database))
	g2.DELETE("/servers/:id", v1.ServersDelete(s.coordinator, s.database))

	g2.GET("/settings", v1.SettingsShow(s.database))
	g2.POST("/settings", v1.SettingsUpdate(s.coordinator, s.database))
	g2.GET("/settings/stats", v1.SettingsStatsShow(s.coordinator, s.database))
	g2.POST("/settings/stats/zero", v1.SettingsStatsZero(s.database))
	g2.POST("/settings/servers/zero", v1.SettingsServersZero(s.database))
	g2.POST("/settings/users/zero", v1.SettingsUsersZero(s.coordinator, s.database))
	g2.POST("/settings/users/delete", v1.SettingsUsersDelete(s.coordinator, s.database))
	g2.POST("/settings/xray/restart", v1.SettingsRestartXray(s.coordinator))

	go func() {
		address := fmt.Sprintf("%s:%d", s.config.HttpServer.Host, s.config.HttpServer.Port)
		if err := s.engine.Start(address); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.l.Exit("http server: failed to start", zap.String("address", address), zap.Error(err))
		}
	}()
}

// Shutdown closes the HTTP Server.
func (s *Server) Shutdown() {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.engine.Shutdown(c); err != nil {
		s.l.Error("http server: failed to close", zap.Error(err))
	} else {
		s.l.Info("http server: closed successfully")
	}
}

// New creates a new instance of HTTP Server.
func New(
	config *config.Config,
	logger *logger.Logger,
	c *coordinator.Coordinator,
	database *database.Database,
	enigma *enigma.Enigma,
) *Server {
	e := echo.New()
	e.HideBanner = true
	e.Validator = validator.New()
	return &Server{
		engine:      e,
		config:      config,
		l:           logger,
		coordinator: c,
		database:    database,
		enigma:      enigma,
	}
}
