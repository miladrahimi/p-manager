package coordinator

import (
	"encoding/json"
	"fmt"
	"github.com/miladrahimi/xray-manager/internal/config"
	"github.com/miladrahimi/xray-manager/internal/database"
	"github.com/miladrahimi/xray-manager/pkg/fetcher"
	"github.com/miladrahimi/xray-manager/pkg/logger"
	"github.com/miladrahimi/xray-manager/pkg/utils"
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
	log      *logger.Logger
	fetcher  *fetcher.Fetcher
	xray     *xray.Xray
}

func (c *Coordinator) Run() {
	c.log.Info("coordinator: running...")
	go c.syncRemoteConfigs()
	go func() {
		for {
			c.log.Info("coordinator: working...")
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
	c.log.Info("coordinator: syncing configs...")
	c.syncLocalConfigs()
	c.syncRemoteConfigs()
}

func (c *Coordinator) syncRemoteConfigs() {
	c.log.Info("coordinator: syncing remote configs...")

	shadowsocksClients := c.generateShadowsocksClients()
	for _, s := range c.database.Data.Servers {
		xc := xray.NewConfig()
		xc.UpdateShadowsocksInbound(shadowsocksClients, s.SsRemotePort)
		go c.updateRemoteConfigs(s, xc)
	}

	c.syncRemoteStats()
}

func (c *Coordinator) syncLocalConfigs() {
	c.log.Info("coordinator: syncing local configs...")

	c.xray.Config().Locker.Lock()
	defer c.xray.Config().Locker.Unlock()

	c.xray.Config().RemoveInbounds()
	for _, s := range c.database.Data.Servers {
		if s.SsLocalPort > 0 {
			c.xray.Config().AddRelayInbound(s.Id, s.Host, s.SsLocalPort, s.SsRemotePort)
		}
	}

	go c.xray.Restart()
}

func (c *Coordinator) SyncStats() {
	c.log.Info("coordinator: syncing stats...")
	c.syncLocalStats()
	c.syncRemoteStats()
}

func (c *Coordinator) syncRemoteStats() {
	c.log.Info("coordinator: syncing remote stats...")
	for _, s := range c.database.Data.Servers {
		go c.fetchRemoteStats(s)
	}
}

func (c *Coordinator) updateRemoteConfigs(s *database.Server, xc *xray.Config) {
	url := fmt.Sprintf("%s://%s:%d/v1/configs", "http", s.Host, s.HttpPort)
	c.log.Info("coordinator: updating remote configs...", zap.String("url", url))

	_, err := c.fetcher.Do("POST", url, s.HttpToken, xc)
	if err != nil {
		c.log.Error("coordinator: cannot update remote configs", zap.Error(err))
	}
}

func (c *Coordinator) fetchRemoteStats(s *database.Server) {
	url := fmt.Sprintf("%s://%s:%d/v1/stats", "http", s.Host, s.HttpPort)
	c.log.Info("coordinator: fetching remote stats", zap.String("url", url))

	c.database.Locker.Lock()
	defer func() {
		c.database.Save()
		c.database.Locker.Unlock()
	}()

	s.Status = database.ServerStatusAvailable

	responseBody, err := c.fetcher.Do("GET", url, s.HttpToken, nil)
	if err != nil {
		c.log.Info("coordinator: cannot fetch remote stats", zap.Error(err))
		s.Status = database.ServerStatusUnavailable
		return
	}

	var qss []*stats.Stat
	if err = json.Unmarshal(responseBody, &qss); err != nil {
		c.log.Error(
			"coordinator: cannot unmarshall fetched query stats body",
			zap.String("url", url),
			zap.Error(err),
			zap.ByteString("body", responseBody),
		)
		s.Status = database.ServerStatusUnavailable
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
			u.Used = utils.RoundFloat(float64(u.UsedBytes)/1000/1000/1000, 2)
			if u.Quota > 0 && u.Used > float64(u.Quota) {
				u.Enabled = false
				isSyncConfigsRequired = true
			}
		}
	}

	if isSyncConfigsRequired {
		go c.SyncConfigs()
	}
}

func (c *Coordinator) DebugSettings() {
	if !c.config.HttpClient.Debug {
		return
	}

	c.log.Info("coordinator: debug internet connection...")

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
	c.log.Info("coordinator: syncing local stats...")

	c.database.Locker.Lock()
	defer c.database.Locker.Unlock()

	for _, s := range c.xray.QueryStats() {
		parts := strings.Split(s.GetName(), ">>>")
		if parts[0] == "inbound" {
			c.database.Data.Stats.Traffic += s.GetValue()
		}
	}

	c.database.Save()
}

func New(c *config.Config, f *fetcher.Fetcher, l *logger.Logger, d *database.Database, x *xray.Xray) *Coordinator {
	return &Coordinator{config: c, log: l, database: d, xray: x, fetcher: f}
}
