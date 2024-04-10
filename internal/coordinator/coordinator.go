package coordinator

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/writer"
	"github.com/miladrahimi/p-manager/pkg/http/client"
	"github.com/miladrahimi/p-manager/pkg/logger"
	"github.com/miladrahimi/p-manager/pkg/utils"
	"github.com/miladrahimi/p-manager/pkg/xray"
	"go.uber.org/zap"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Coordinator struct {
	l        *logger.Logger
	context  context.Context
	config   *config.Config
	database *database.Database
	hc       *client.Client
	xray     *xray.Xray
	writer   *writer.Writer
}

func (c *Coordinator) Run() {
	c.l.Info("coordinator: running...")

	c.SyncConfigs()

	go newWorker(c.context, time.Duration(c.config.Worker.Interval)*time.Second, func() {
		c.l.Info("coordinator: running stats worker...")
		c.SyncStats()
	}).Start()

	go newWorker(c.context, time.Minute, func() {
		c.l.Info("coordinator: running node worker...")
		c.syncOutdatedConfigs()
	}).Start()

	go newWorker(c.context, time.Hour, func() {
		c.l.Info("coordinator: running backup worker...")
		c.database.Backup()
	}).Start()
}

func (c *Coordinator) SyncConfigs() {
	c.l.Info("coordinator: syncing configs...")
	c.syncLocalConfig()
	c.syncRemoteConfigs()
}

func (c *Coordinator) syncLocalConfig() {
	c.l.Info("coordinator: syncing local configs...")
	c.xray.SetConfig(c.writer.LocalConfig())
	c.xray.Restart()
}

func (c *Coordinator) syncRemoteConfigs() {
	c.l.Info("coordinator: syncing remote configs...")
	for _, s := range c.database.Data.Servers {
		go c.syncRemoteConfig(s, c.writer.RemoteConfig(s))
	}
}

func (c *Coordinator) syncOutdatedConfigs() {
	c.l.Info("coordinator: syncing outdated configs...")
	for _, s := range c.database.Data.Servers {
		if s.Status != database.ServerStatusAvailable {
			go c.syncRemoteConfig(s, c.writer.RemoteConfig(s))
		}
	}
}

func (c *Coordinator) syncRemoteConfig(s *database.Server, xc *xray.Config) {
	url := fmt.Sprintf("%s://%s:%d/v1/configs", "http", s.Host, s.HttpPort)
	c.l.Info("coordinator: syncing remote config...", zap.String("url", url))

	_, err := c.hc.Do(http.MethodPost, url, xc, map[string]string{
		echo.HeaderAuthorization: fmt.Sprintf("Bearer %s", s.HttpToken),
	})
	if err != nil {
		c.l.Error("coordinator: cannot sync remote config", zap.Error(err), zap.String("url", url))
		s.Status = database.ServerStatusUnavailable
	} else {
		s.Status = database.ServerStatusAvailable
		c.l.Debug("coordinator: remote config synced", zap.String("url", url))
	}
}

func (c *Coordinator) SyncStats() {
	c.l.Info("coordinator: syncing stats...")

	queryStats := c.xray.QueryStats()

	c.database.Locker.Lock()
	defer c.database.Locker.Unlock()

	servers := map[string]int64{}
	users := map[string]int64{}

	for _, qs := range queryStats {
		parts := strings.Split(qs.GetName(), ">>>")
		if parts[0] == "user" {
			users[parts[1]] += qs.GetValue()
		} else if parts[0] == "inbound" && strings.HasPrefix(parts[1], "foreign-") {
			servers[parts[1][8:]] += qs.GetValue()
		} else if parts[0] == "outbound" && strings.HasPrefix(parts[1], "relay-") {
			servers[parts[1][6:]] += qs.GetValue()
		} else if parts[0] == "inbound" && slices.Contains([]string{"reverse", "relay", "direct"}, parts[1]) {
			c.database.Data.Stats.Traffic += float64(qs.GetValue()) / 1000 / 1000 / 1000
		}
	}

	for _, s := range c.database.Data.Servers {
		if bytes, found := servers[strconv.Itoa(s.Id)]; found {
			s.Traffic += utils.RoundFloat(float64(bytes)/1000/1000/1000, 2)
		}
	}

	shouldSync := false
	for _, u := range c.database.Data.Users {
		if bytes, found := users[strconv.Itoa(u.Id)]; found {
			u.UsedBytes += bytes
			u.Used = utils.RoundFloat(float64(u.UsedBytes)/1000/1000/1000, 2)
			if u.Quota > 0 && u.Used > u.Quota {
				u.Enabled = false
				shouldSync = true
			}
		}
	}

	if shouldSync {
		go c.SyncConfigs()
	}

	c.database.Save()
}

func New(
	config *config.Config,
	context context.Context,
	hc *client.Client,
	logger *logger.Logger,
	database *database.Database,
	xray *xray.Xray,
	writer *writer.Writer,
) *Coordinator {
	return &Coordinator{
		l:        logger,
		hc:       hc,
		config:   config,
		context:  context,
		database: database,
		xray:     xray,
		writer:   writer,
	}
}
