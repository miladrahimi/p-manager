package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
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

	// User APIs (account holder; no admin auth, reached via the account link)
	userApi := s.engine.Group("api/user")
	userApi.GET("/account/:accountId", api.AccountShow(s.composer, s.db))
	userApi.POST("/account/:accountId/links/renew", api.AccountLinksRenew(s.coordinator, s.db))

	// Admin APIs
	adminApi := s.engine.Group("api/admin")
	adminApi.POST("/sign-in", api.SignIn(s.db))
	adminApi.Use(s.authorizeAdmin)

	// Admin APIs: Accounts
	adminApi.GET("/accounts", api.AccountsIndex(s.db))
	adminApi.POST("/accounts", api.AccountsStore(s.coordinator, s.db))
	adminApi.PATCH("/accounts", api.AccountsUpdatePartialBatch(s.coordinator, s.db))
	adminApi.PUT("/accounts/:id", api.AccountsUpdate(s.coordinator, s.db))
	adminApi.PATCH("/accounts/:id", api.AccountsUpdatePartial(s.coordinator, s.db))
	adminApi.DELETE("/accounts/:id", api.AccountsDelete(s.coordinator, s.db))
	adminApi.DELETE("/accounts", api.AccountsDeleteBatch(s.coordinator, s.db))
	adminApi.POST("/accounts/import", api.AccountsImport(s.coordinator, s.db, s.hc))

	// Admin APIs: Nodes
	adminApi.GET("/nodes", api.NodesIndex(s.db))
	adminApi.POST("/nodes", api.NodesStore(s.coordinator, s.db))
	adminApi.PATCH("/nodes", api.NodesUpdatePartialBatch(s.coordinator, s.db))
	adminApi.PUT("/nodes/:id", api.NodesUpdate(s.coordinator, s.db))
	adminApi.PATCH("/nodes/:id", api.NodesUpdateToggles(s.coordinator, s.db))
	adminApi.DELETE("/nodes/:id", api.NodesDelete(s.coordinator, s.db))

	// Admin APIs: Stats
	adminApi.GET("/stats", api.StatsIndex(s.db))
	adminApi.PATCH("/stats", api.StatsUpdatePartial(s.db))

	// Admin APIs: Platform
	adminApi.GET("/platform", api.PlatformShow())

	// Admin APIs: Insights
	adminApi.GET("/insights", api.InsightsIndex(s.db))

	// Admin APIs: Main Settings
	adminApi.GET("/main-settings", api.MainSettingsShow(s.db))
	adminApi.POST("/main-settings", api.MainSettingsUpdate(s.db))

	// Admin APIs: Xray
	adminApi.POST("/xray/restart", api.XrayRestart(s.coordinator))
	adminApi.GET("/xray-settings", api.XraySettingsShow(s.db))
	adminApi.POST("/xray-settings", api.XraySettingsUpdate(s.coordinator, s.db))

	// Node APIs (each node authenticates with its own pull token)
	nodeApi := s.engine.Group("api/node")
	nodeApi.Use(s.authorizeNode)
	nodeApi.GET("/:id/config", api.NodesConfigShow(s.coordinator, s.composer, s.db))

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

// authorizeAdmin authorizes a request against the admin password.
func (s *Server) authorizeAdmin(next echo.HandlerFunc) echo.HandlerFunc {
	return cm.Authorize(func() string {
		var token string
		s.db.Read(func(d *data.Data) {
			token = d.MainSettings.AdminPassword
		})
		return token
	})(next)
}

// authorizeNode authorizes a request against the related node token.
func (s *Server) authorizeNode(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		auth := c.Request().Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			return echo.ErrUnauthorized
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		id := c.Param("id")

		authorized := false
		s.db.Read(func(d *data.Data) {
			for _, n := range d.Nodes {
				if n.Id == id && n.PullToken != "" &&
					subtle.ConstantTimeCompare([]byte(n.PullToken), []byte(token)) == 1 {
					authorized = true
					return
				}
			}
		})
		if !authorized {
			return echo.ErrUnauthorized
		}
		return next(c)
	}
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
