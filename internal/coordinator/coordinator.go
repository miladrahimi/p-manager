package coordinator

import (
	"context"
	"fmt"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/internal/utils"
	"github.com/miladrahimi/p-manager/internal/writer"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
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

	go newWorker(c.context, time.Duration(c.config.Workers.SyncStatsInterval)*time.Second, func() {
		c.l.Info("coordinator: running worker for sync stats...")
		c.SyncStats()
	}, func() {
		c.l.Debug("coordinator: worker for sync stats stopped")
	}).Start()

	go newWorker(c.context, time.Minute, func() {
		c.l.Info("coordinator: running worker to sync outdated configs...")
		c.syncOutdatedConfigs()
	}, func() {
		c.l.Debug("coordinator: worker for sync outdated configs stopped")
	}).Start()

	go newWorker(c.context, time.Hour, func() {
		c.l.Info("coordinator: running worker to backup database...")
		c.database.Backup()
	}, func() {
		c.l.Debug("coordinator: worker for backup database stopped")
	}).Start()

	go newWorker(c.context, time.Hour, func() {
		c.l.Info("coordinator: running worker to reset users...")
		c.resetUsers()
	}, func() {
		c.l.Debug("coordinator: worker for reset users stopped")
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
	for _, s := range c.database.Content.Nodes {
		go c.syncRemoteConfig(s, c.writer.RemoteConfig(s))
	}
}

func (c *Coordinator) syncOutdatedConfigs() {
	c.l.Info("coordinator: syncing outdated configs...")
	for _, s := range c.database.Content.Nodes {
		if s.Status == database.NodeStatusUnavailable || s.Status == database.NodeStatusProcessing {
			go c.syncRemoteConfig(s, c.writer.RemoteConfig(s))
		}
	}
}

func (c *Coordinator) syncRemoteConfig(s *database.Node, xc *xray.Config) {
	url := fmt.Sprintf("%s://%s:%d/v1/configs", "http", s.Host, s.HttpPort)
	proxy := c.database.Content.Settings.SingetServer
	proxied := false
	success := false

	c.l.Info("coordinator: syncing remote config...", zap.String("url", url), zap.String("proxy", proxy))

	_, err := c.hc.Do(http.MethodPost, url, s.HttpToken, xc)
	if err == nil {
		success = true
	} else if proxy != "" {
		proxied = true
		_, err = c.hc.DoThrough(proxy, http.MethodPost, url, s.HttpToken, xc)
		if err == nil {
			success = true
		}
	}

	if success {
		if proxied {
			s.Status = database.NodeStatusDirty
		} else {
			s.Status = database.NodeStatusAvailable
		}
		c.l.Debug(
			"coordinator: remote config synced",
			zap.String("url", url),
			zap.String("proxy", proxy),
			zap.Bool("proxied", proxied),
		)
	} else {
		s.Status = database.NodeStatusUnavailable
		c.l.Error(
			"coordinator: cannot sync remote config",
			zap.String("url", url),
			zap.String("proxy", proxy),
			zap.Bool("proxied", proxied),
			zap.Error(err),
		)
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
			c.database.Content.Stats.TotalUsage += float64(qs.GetValue()) / 1000 / 1000 / 1000
		}
	}

	for _, s := range c.database.Content.Nodes {
		if bytes, found := servers[strconv.Itoa(s.Id)]; found {
			s.Usage += utils.RoundFloat(float64(bytes)/1000/1000/1000, 2)
		}
	}

	shouldSync := false
	for _, u := range c.database.Content.Users {
		if bytes, found := users[strconv.Itoa(u.Id)]; found {
			u.UsedBytes += bytes
			u.Used = utils.RoundFloat(float64(u.UsedBytes)/1000/1000/1000, 2)
			if u.Quota > 0 && u.Used > u.Quota {
				u.Enabled = false
				shouldSync = true
				c.l.Debug("coordinator: user disabled", zap.Int("id", u.Id))
			}
		}
	}

	if shouldSync {
		go c.SyncConfigs()
	}

	c.database.Save()
}

func (c *Coordinator) resetUsers() {
	if c.database.Content.Settings.ResetPolicy != "monthly" {
		return
	}

	c.l.Info("coordinator: resetting users...")

	for _, u := range c.database.Content.Users {
		if time.Unix(u.UsageResetAt, 0).Format("2006-01") == time.Now().Format("2006-01") {
			continue
		}
		u.Used = 0
		u.UsedBytes = 0
		u.Enabled = true
		u.UsageResetAt = time.Now().Unix()
	}

	c.database.Save()
	go c.SyncConfigs()
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
