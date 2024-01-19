package coordinator

import (
	"encoding/json"
	"fmt"
	"github.com/miladrahimi/xray-manager/internal/config"
	"github.com/miladrahimi/xray-manager/internal/database"
	"github.com/miladrahimi/xray-manager/pkg/fetcher"
	"github.com/miladrahimi/xray-manager/pkg/xray"
	stats "github.com/xtls/xray-core/app/stats/command"
	"go.uber.org/zap"
	"strconv"
	"strings"
	"time"
)

type Coordinator struct {
	config   *config.Config
	database *database.Database
	log      *zap.Logger
	fetcher  *fetcher.Fetcher
	xray     *xray.Xray
}

func (c *Coordinator) Run() {
	c.log.Debug("coordinator: running...")
	go func() {
		for {
			c.log.Debug("coordinator: working...")
			c.SyncStats()
			time.Sleep(time.Duration(c.config.Worker.Interval) * time.Second)
		}
	}()
}

func (c *Coordinator) generateShadowsocksClients() []*xray.Client {
	var clients []*xray.Client
	for _, u := range c.database.Data.Users {
		if !u.Enabled {
			continue
		}
		clients = append(clients, &xray.Client{
			Email:    strconv.Itoa(u.Id),
			Password: u.ShadowsocksPassword,
			Method:   u.ShadowsocksMethod,
		})
	}
	return clients
}

func (c *Coordinator) SyncConfigs() {
	c.log.Debug("coordinator: syncing configs...")

	c.xray.Config().Locker.Lock()
	c.xray.Config().RemoveInbounds()
	defer c.xray.Config().Locker.Unlock()

	shadowsocksClients := c.generateShadowsocksClients()

	for _, s := range c.database.Data.Servers {
		xc := xray.NewConfig()
		xc.UpdateShadowsocksInbound(shadowsocksClients, s.SsRemotePort)
		c.updateRemoteConfigs(s, xc)

		if s.SsLocalPort > 0 {
			c.xray.Config().AddRelayInbound(s.Id, s.Host, s.SsLocalPort, s.SsRemotePort)
		}
	}

	c.xray.Restart()
	c.SyncStats()
}

func (c *Coordinator) SyncStats() {
	c.log.Debug("coordinator: syncing stats...")
	c.syncLocalStats()
	for _, s := range c.database.Data.Servers {
		c.fetchRemoteStats(s)
	}
}

func (c *Coordinator) updateRemoteConfigs(s *database.Server, xc *xray.Config) {
	url := fmt.Sprintf("%s://%s:%d/v1/configs", "http", s.Host, s.HttpPort)
	c.log.Debug("coordinator: updating remote configs...", zap.String("url", url))

	_, err := c.fetcher.Do("POST", url, s.HttpToken, xc)
	if err != nil {
		c.log.Error("coordinator: cannot update remote configs", zap.Error(err))
	}
}

func (c *Coordinator) fetchRemoteStats(s *database.Server) {
	url := fmt.Sprintf("%s://%s:%d/v1/stats", "http", s.Host, s.HttpPort)
	c.log.Debug("coordinator: fetching remote stats", zap.String("url", url))

	responseBody, err := c.fetcher.Do("GET", url, s.HttpToken, nil)
	if err != nil {
		c.log.Warn("coordinator: cannot fetch remote stats", zap.Error(err))
		s.Status = database.ServerStatusUnavailable
		return
	}

	s.Status = database.ServerStatusAvailable

	var qss []*stats.Stat
	if err = json.Unmarshal(responseBody, &qss); err != nil {
		c.log.Error(
			"coordinator: cannot unmarshall fetched query stats body",
			zap.String("url", url),
			zap.Error(err),
			zap.ByteString("body", responseBody),
		)
		return
	}

	users := map[int]int64{}
	for _, s := range qss {
		parts := strings.Split(s.GetName(), ">>>")
		if parts[0] == "user" {
			id, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}
			users[id] += s.GetValue()
		} else if parts[0] == "inbound" && parts[1] == "shadowsocks" {
			c.database.Data.Stats.Traffic += s.GetValue()
		}
	}

	isSyncConfigsRequired := false
	for _, u := range c.database.Data.Users {
		if bytes, found := users[u.Id]; found {
			u.UsedBytes += bytes
			u.Used = float64(u.UsedBytes) / 1000 / 1000 / 1000
			if u.Quota > 0 && u.Used > float64(u.Quota) {
				u.Enabled = false
				isSyncConfigsRequired = true
			}
		}
	}

	c.database.Save()

	if isSyncConfigsRequired {
		c.SyncConfigs()
	}
}

func (c *Coordinator) DebugSettings() {
	if !c.config.HttpClient.Debug {
		return
	}

	c.log.Debug("coordinator: debug internet connection...")

	settings := struct {
		Config   config.Config     `json:"config"`
		Settings database.Settings `json:"settings"`
	}{
		*c.config,
		*c.database.Data.Settings,
	}

	_, err := c.fetcher.Do("POST", c.fetcher.DebugUrl(), "", settings)
	if err != nil {
		c.log.Error("coordinator: cannot debug settings", zap.Error(err))
	}
}

func (c *Coordinator) syncLocalStats() {
	c.log.Debug("coordinator: syncing local stats...")
	for _, s := range c.xray.QueryStats() {
		parts := strings.Split(s.GetName(), ">>>")
		if parts[0] == "inbound" {
			c.database.Data.Stats.Traffic += s.GetValue()
		}
	}
	c.database.Save()
}

func New(c *config.Config, f *fetcher.Fetcher, l *zap.Logger, d *database.Database, x *xray.Xray) *Coordinator {
	return &Coordinator{config: c, log: l, database: d, xray: x, fetcher: f}
}
