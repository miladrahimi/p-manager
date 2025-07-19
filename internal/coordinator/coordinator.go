package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/internal/http/client"
	"github.com/miladrahimi/p-manager/internal/utils"
	"github.com/miladrahimi/p-manager/internal/writer"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
	"github.com/xtls/xray-core/app/stats/command"
	"go.uber.org/zap"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Coordinator struct {
	l       *logger.Logger
	context context.Context
	config  *config.Config
	d       *database.Database
	hc      *client.Client
	xray    *xray.Xray
	writer  *writer.Writer
	state   *State
}

func (c *Coordinator) Run() {
	c.l.Info("coordinator: running...")

	c.SyncConfigs()

	go newWorker(c.context, time.Second*10, func() {
		c.l.Info("coordinator: running worker to sync outdated configs...")
		c.syncOutdatedConfigs()
	}, func() {
		c.l.Debug("coordinator: worker for sync outdated configs stopped")
	}).Start()

	go newWorker(c.context, time.Minute, func() {
		c.l.Info("coordinator: running worker for pull statuses...")
		if err := c.syncNodePullStatuses(); err != nil {
			c.l.Error("coordinator: cannot pull statuses", zap.Error(errors.WithStack(err)))
		}
	}, func() {
		c.l.Debug("coordinator: worker for pull statuses stopped")
	}).Start()

	go newWorker(c.context, time.Minute, func() {
		c.l.Info("coordinator: running worker for sync local stats...")
		if err := c.syncLocalStats(); err != nil {
			c.l.Error("coordinator: cannot sync local stats", zap.Error(errors.WithStack(err)))
		}
	}, func() {
		c.l.Debug("coordinator: worker for sync stats stopped")
	}).Start()

	go newWorker(c.context, time.Minute, func() {
		c.l.Info("coordinator: running worker for sync remote stats...")
		c.syncRemoteStats()
	}, func() {
		c.l.Debug("coordinator: worker for sync stats stopped")
	}).Start()

	go newWorker(c.context, time.Hour, func() {
		c.l.Info("coordinator: running worker to backup d...")
		c.d.Backup()
	}, func() {
		c.l.Debug("coordinator: worker for backup d stopped")
	}).Start()

	go newWorker(c.context, time.Hour, func() {
		c.l.Info("coordinator: running worker to reset users...")
		if err := c.resetUserUsages(); err != nil {
			c.l.Error("coordinator: cannot reset users usages", zap.Error(errors.WithStack(err)))
		}
	}, func() {
		c.l.Debug("coordinator: worker for reset users stopped")
	}).Start()
}

func (c *Coordinator) SyncConfigs() {
	c.l.Info("coordinator: syncing configs...")
	if err := c.syncLocalConfig(); err != nil {
		c.l.Fatal("coordinator: cannot sync local configs", zap.Error(errors.WithStack(err)))
	}
	c.syncRemoteConfigs()
}

func (c *Coordinator) syncLocalConfig() error {
	c.l.Info("coordinator: syncing local configs...")

	localConfig, err := c.writer.LocalConfig()
	if err != nil {
		return err
	}

	c.state.xrayUpdatedAt = time.Now()

	c.xray.SetConfig(localConfig)
	c.xray.Restart()

	return nil
}

func (c *Coordinator) syncRemoteConfigs() {
	c.l.Info("coordinator: syncing remote configs...")
	for _, s := range c.d.Content.Nodes {
		go c.syncRemoteConfig(s)
	}
}

func (c *Coordinator) syncOutdatedConfigs() {
	c.l.Info("coordinator: syncing outdated configs...")
	for _, n := range c.d.Content.Nodes {
		if n.PushStatus == database.NodeStatusUnavailable || n.PushStatus == database.NodeStatusProcessing {
			go c.syncRemoteConfig(n)
		}
	}
}

func (c *Coordinator) syncRemoteConfig(node *database.Node) {
	url := fmt.Sprintf("%s://%s:%d/v1/configs", "http", node.Host, node.HttpPort)
	proxy := c.d.Content.Settings.SingetServer
	proxied := false
	success := false

	xc := c.writer.RemoteConfig(node, c.state.XrayUpdatedAt(), c.state.XraySharedPassword())

	c.l.Info("coordinator: syncing remote config...", zap.String("url", url), zap.String("proxy", proxy))

	_, err := c.hc.Do(http.MethodPost, url, node.HttpToken, xc)
	if err == nil {
		success = true
	} else if proxy != "" {
		proxied = true
		_, err = c.hc.DoThrough(proxy, http.MethodPost, url, node.HttpToken, xc)
		if err == nil {
			success = true
		}
	}

	if success {
		node.PushedAt = time.Now().UnixMilli()
		if proxied {
			node.PushStatus = database.NodeStatusDirty
		} else {
			node.PushStatus = database.NodeStatusAvailable
		}

		c.l.Debug(
			"coordinator: remote config synced",
			zap.String("url", url),
			zap.String("proxy", proxy),
			zap.Bool("proxied", proxied),
		)
	} else {
		node.PushStatus = database.NodeStatusUnavailable
		c.l.Error(
			"coordinator: cannot sync remote config",
			zap.String("url", url),
			zap.String("proxy", proxy),
			zap.Bool("proxied", proxied),
			zap.Error(err),
		)
	}
}

func (c *Coordinator) syncRemoteStats() {
	if c.d.Content.Settings.SsRemotePort == 0 {
		c.l.Debug("coordinator: remote stats disabled")
		return
	}

	c.l.Info("coordinator: syncing remote stats...")
	for _, s := range c.d.Content.Nodes {
		go c.syncRemoteNodeStats(s)
	}
}

func (c *Coordinator) syncRemoteNodeStats(node *database.Node) {
	url := fmt.Sprintf("%s://%s:%d/v1/stats", "http", node.Host, node.HttpPort)

	c.l.Info("coordinator: syncing remote node stats...", zap.String("url", url))

	response, err := c.hc.Do(http.MethodGet, url, node.HttpToken, nil)
	if err != nil {
		c.l.Error("cannot sync remote node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
		return
	}

	var queryStats []*command.Stat
	if err = json.Unmarshal(response, &queryStats); err != nil {
		c.l.Error("cannot read remote node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
		return
	}

	c.d.Locker.Lock()
	defer c.d.Locker.Unlock()

	users := map[string]int64{}
	var nodeUsageBytes int64

	for _, qs := range queryStats {
		parts := strings.Split(qs.GetName(), ">>>")
		if parts[0] == "user" {
			users[parts[1]] += qs.GetValue()
		} else if parts[0] == "inbound" && parts[1] == "remote" {
			nodeUsageBytes += qs.GetValue()
		}
	}

	shouldSync := false
	for _, u := range c.d.Content.Users {
		if bytes, found := users[strconv.Itoa(u.Id)]; found {
			u.UsageBytes = utils.SafeSumI64(u.UsageBytes, bytes)
			u.Usage = utils.Bytes2GB(u.UsageBytes)
			if u.Quota > 0 && u.Usage > u.Quota {
				u.Enabled = false
				shouldSync = true
				c.l.Debug("coordinator: user disabled", zap.Int("id", u.Id))
			}
		}
	}
	if shouldSync {
		go c.SyncConfigs()
	}

	node.UsageBytes = utils.SafeSumI64(node.UsageBytes, nodeUsageBytes)
	node.Usage = utils.Bytes2GB(node.UsageBytes)

	c.d.Content.Stats.TotalUsageBytes = utils.SafeSumI64(c.d.Content.Stats.TotalUsageBytes, nodeUsageBytes)
	c.d.Content.Stats.TotalUsage = utils.Bytes2GB(c.d.Content.Stats.TotalUsageBytes)

	if err = c.d.Save(); err != nil {
		c.l.Error("cannot save remote node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
	}
}

func (c *Coordinator) syncLocalStats() error {
	c.l.Info("coordinator: syncing local stats...")

	queryStats, err := c.xray.QueryStats()
	if err != nil {
		return errors.WithStack(err)
	}

	c.d.Locker.Lock()
	defer c.d.Locker.Unlock()

	nodes := map[string]int64{}
	users := map[string]int64{}

	for _, qs := range queryStats {
		parts := strings.Split(qs.GetName(), ">>>")
		if parts[0] == "user" {
			users[parts[1]] += qs.GetValue()
		} else if parts[0] == "inbound" && strings.HasPrefix(parts[1], "internal-") {
			nodes[parts[1][8:]] += qs.GetValue()
		} else if parts[0] == "outbound" && strings.HasPrefix(parts[1], "relay-") {
			nodes[parts[1][6:]] += qs.GetValue()
		} else if parts[0] == "inbound" && slices.Contains([]string{"reverse", "relay", "direct"}, parts[1]) {
			c.d.Content.Stats.TotalUsageBytes = utils.SafeSumI64(c.d.Content.Stats.TotalUsageBytes, qs.GetValue())
		}
	}

	for _, n := range c.d.Content.Nodes {
		if bytes, found := nodes[strconv.Itoa(n.Id)]; found {
			n.UsageBytes = utils.SafeSumI64(n.UsageBytes, bytes)
		}
		n.Usage = utils.Bytes2GB(n.UsageBytes)
	}

	c.d.Content.Stats.TotalUsage = utils.Bytes2GB(c.d.Content.Stats.TotalUsageBytes)

	shouldSync := false
	for _, u := range c.d.Content.Users {
		if bytes, found := users[strconv.Itoa(u.Id)]; found {
			u.UsageBytes = utils.SafeSumI64(u.UsageBytes, bytes)
			u.Usage = utils.Bytes2GB(u.UsageBytes)
			if u.Quota > 0 && u.Usage > u.Quota {
				u.Enabled = false
				shouldSync = true
				c.l.Debug("coordinator: user disabled", zap.Int("id", u.Id))
			}
		}
	}
	if shouldSync {
		go c.SyncConfigs()
	}

	err = c.d.Save()
	return errors.WithStack(err)
}

func (c *Coordinator) syncNodePullStatuses() error {
	c.l.Info("coordinator: syncing pull statuses...")

	needsSync := false
	for _, n := range c.d.Content.Nodes {
		if time.Now().Sub(time.UnixMilli(n.PulledAt)) > time.Minute && n.PullStatus != database.NodeStatusUnavailable {
			c.l.Info(fmt.Sprintf("Node %d marked as unavailable", n.Id))
			n.PullStatus = database.NodeStatusUnavailable
			needsSync = true
		}
	}

	if needsSync {
		err := c.d.Save()
		return errors.WithStack(err)
	}

	return nil
}

func (c *Coordinator) resetUserUsages() error {
	if c.d.Content.Settings.ResetPolicy != "monthly" {
		return nil
	}

	c.l.Info("coordinator: resetting users usages...")

	for _, u := range c.d.Content.Users {
		if time.Unix(u.UsageResetAt, 0).Format("2006-01") == time.Now().Format("2006-01") {
			continue
		}
		u.Usage = 0
		u.UsageBytes = 0
		u.Enabled = true
		u.UsageResetAt = time.Now().Unix()
	}

	if err := c.d.Save(); err != nil {
		return errors.WithStack(err)
	}

	go c.SyncConfigs()

	return nil
}

func (c *Coordinator) State() *State {
	return c.state
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
		l:       logger,
		hc:      hc,
		config:  config,
		context: context,
		d:       database,
		xray:    xray,
		writer:  writer,
		state:   NewState(),
	}
}
