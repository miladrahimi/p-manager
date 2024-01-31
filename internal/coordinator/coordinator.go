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
	"os"
	"strconv"
	"strings"
	"time"
)

type Coordinator struct {
	config   *config.Config
	database *database.Database
	log      *logger.Logger
	fetcher  *fetcher.Fetcher
	xray     *xray.Portal
}

func (c *Coordinator) Run() {
	c.log.Info("coordinator: running...")

	c.syncDatabase()
	c.SyncConfigs()

	statsWorker := time.NewTicker(time.Duration(c.config.Worker.Interval) * time.Second)
	go func() {
		for {
			c.log.Info("coordinator: working...")
			c.SyncStats()
			<-statsWorker.C
		}
	}()

	backupWorker := time.NewTicker(3 * time.Hour)
	go func() {
		for {
			<-backupWorker.C
			c.log.Info("coordinator: backing up...")
			c.database.Backup()
		}
	}()
}

func (c *Coordinator) syncDatabase() {
	if c.xray.Config().SspInbound() != nil {
		c.database.Data.Settings.SspPort = c.xray.Config().SspInbound().Port
	}
	if c.xray.Config().SsdInbound() != nil {
		c.database.Data.Settings.SsdPort = c.xray.Config().SsdInbound().Port
	}
	if len(c.database.Data.Servers) > 1 {
		c.database.Data.Servers = []*database.Server{c.database.Data.Servers[0]}
	}
	c.database.Save()
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

func (c *Coordinator) syncLocalConfigs() {
	c.log.Info("coordinator: syncing local configs...")

	clients := c.generateShadowsocksClients()
	c.xray.Config().SspInbound()
	c.xray.Config().UpdateSspInbound(clients, c.database.Data.Settings.SspPort)
	c.xray.Config().UpdateSsdInbound(clients, c.database.Data.Settings.SsdPort)

	for i, s := range c.database.Data.Servers {
		c.xray.Config().SsdOutbound().Settings.Servers[i].Address = s.Host
	}

	go c.xray.Restart()
}

func (c *Coordinator) syncRemoteConfigs() {
	c.log.Info("coordinator: syncing remote configs...")

	xc := xray.NewBridgeConfig()
	xc.UpdateReverseOutbound(
		c.database.Data.Settings.Host,
		c.xray.Config().ReverseInbound().Port,
		c.xray.Config().ReverseInbound().Settings.Password,
	)

	for _, s := range c.database.Data.Servers {
		xc.SsdInbound().Port = c.xray.Config().SsdOutbound().Settings.Servers[0].Port
		xc.SsdInbound().Settings.Clients[0].Password = c.xray.Config().SsdOutbound().Settings.Servers[0].Password
		xc.SsdInbound().Settings.Clients[0].Method = c.xray.Config().SsdOutbound().Settings.Servers[0].Method
		go c.updateRemoteConfigs(s, xc)

		j, _ := json.Marshal(xc)
		_ = os.WriteFile("test.json", j, 0777)
	}

	c.syncRemoteStats()
}

func (c *Coordinator) SyncStats() {
	c.log.Info("coordinator: syncing stats...")
	c.syncLocalStats()
	c.syncRemoteStats()
}

func (c *Coordinator) updateRemoteConfigs(s *database.Server, xc *xray.Config) {
	url := fmt.Sprintf("%s://%s:%d/v1/configs", "http", s.Host, s.HttpPort)
	c.log.Info("coordinator: updating remote configs...", zap.String("url", url))

	_, err := c.fetcher.Do("POST", url, s.HttpToken, xc)
	if err != nil {
		c.log.Error("coordinator: cannot update remote configs", zap.Error(err))
	}
}

func (c *Coordinator) syncRemoteStats() {
	c.log.Info("coordinator: syncing remote stats...")
	for _, s := range c.database.Data.Servers {
		go c.fetchRemoteStats(s)
	}
}

func (c *Coordinator) fetchRemoteStats(s *database.Server) {
	url := fmt.Sprintf("%s://%s:%d/v1/stats", "http", s.Host, s.HttpPort)
	c.log.Info("coordinator: fetching remote stats", zap.String("url", url))

	defer c.database.Save()

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

	for _, qs := range qss {
		parts := strings.Split(qs.GetName(), ">>>")
		if parts[0] == "outbound" && parts[1] == "reverse" {
			s.Traffic += float64(qs.GetValue()) / 1000 / 1000 / 1000
		}
	}
}

func (c *Coordinator) syncLocalStats() {
	c.log.Info("coordinator: syncing local stats...")

	users := map[int]int64{}

	for _, qs := range c.xray.QueryStats() {
		parts := strings.Split(qs.GetName(), ">>>")
		if parts[0] == "user" {
			id, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}
			users[id] += qs.GetValue()
		} else if parts[0] == "inbound" {
			c.database.Data.Stats.Traffic += float64(qs.GetValue()) / 1000 / 1000 / 1000
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

	c.database.Save()
}

func (c *Coordinator) Report() {
	if !c.config.Report {
		return
	}

	c.log.Info("coordinator: reporting information...")

	settings := struct {
		Config   config.Config     `json:"config"`
		Settings database.Settings `json:"settings"`
	}{
		*c.config,
		*c.database.Data.Settings,
	}

	_, err := c.fetcher.Do("POST", "https://rg.miladrahimi.com", "", settings)
	if err != nil {
		c.log.Error("coordinator: cannot debug settings", zap.Error(err))
	}
}

func New(c *config.Config, f *fetcher.Fetcher, l *logger.Logger, d *database.Database, x *xray.Portal) *Coordinator {
	return &Coordinator{config: c, log: l, database: d, xray: x, fetcher: f}
}
