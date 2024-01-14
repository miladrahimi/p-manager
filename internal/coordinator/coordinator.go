package coordinator

import (
	"bytes"
	"encoding/json"
	"github.com/labstack/gommon/random"
	"go.uber.org/zap"
	"io"
	"net/http"
	"shadowsocks-manager/internal/config"
	"shadowsocks-manager/internal/database"
	"shadowsocks-manager/internal/http/client"
	"shadowsocks-manager/internal/utils"
	"shadowsocks-manager/internal/xray"
	"strconv"
	"strings"
	"time"
)

type Coordinator struct {
	config   *config.Config
	database *database.Database
	log      *zap.Logger
	hc       *http.Client
	xray     *xray.Xray
}

func (c *Coordinator) Run() {
	c.log.Debug("coordinator: running...")
	go func() {
		for {
			c.log.Debug("coordinator: worker running...")
			c.syncStats()
			time.Sleep(time.Duration(c.config.Worker.Interval) * time.Second)
		}
	}()
}

func (c *Coordinator) SyncUsers() {
	c.log.Debug("coordinator: syncing users...")

	var clients []xray.Client
	for _, u := range c.database.Data.Users {
		if !u.Enabled {
			continue
		}
		clients = append(clients, xray.Client{
			Email:    strconv.Itoa(u.Id),
			Password: u.Password,
			Method:   u.Method,
		})
	}

	if len(clients) == 0 {
		clients = append(clients, xray.Client{
			Email:    strconv.Itoa(1),
			Password: random.String(16),
			Method:   config.ShadowsocksMethod,
		})
	}

	c.xray.UpdateClients(clients)
}

func (c *Coordinator) SyncUsersAndStats() {
	c.log.Debug("coordinator: syncing users and stats...")
	c.SyncUsers()
	c.syncXrayStats()
}

func (c *Coordinator) SyncServers() {
	c.log.Debug("coordinator: syncing servers...")

	var servers []xray.Server
	for _, s := range c.database.Data.Servers {
		if s.Status == database.ServerStatusProcessing || s.Status == database.ServerStatusUnavailable {
			continue
		}
		servers = append(servers, xray.Server{
			Address:  s.Host,
			Port:     s.Port,
			Method:   s.Method,
			Password: s.Password,
		})
	}

	if len(servers) == 0 {
		if len(c.database.Data.Servers) > 0 {
			s := c.database.Data.Servers[len(c.database.Data.Servers)-1]
			servers = append(servers, xray.Server{
				Address:  s.Host,
				Port:     s.Port,
				Method:   s.Method,
				Password: s.Password,
			})
		} else {
			servers = append(servers, xray.Server{
				Address:  "127.0.0.1",
				Port:     1919,
				Method:   config.ShadowsocksMethod,
				Password: "password",
			})
		}
	}

	c.xray.UpdateServers(servers)
}

func (c *Coordinator) SyncServersAndStats() {
	c.log.Debug("coordinator: syncing servers and stats...")

	c.SyncServers()
	c.syncServerStats()
}

func (c *Coordinator) SyncSettings() {
	c.log.Debug("coordinator: syncing settings...")
	c.testInternetConnection()
	c.xray.UpdateInboundPort(c.database.Data.Settings.ShadowsocksPort)
}

func (c *Coordinator) testInternetConnection() {
	jsonData, err := json.Marshal(c.testInternetConfig())
	if err != nil {
		c.log.Error("coordinator: cannot marshal test data", zap.Error(err))
		return
	}

	req, err := http.NewRequest("POST", client.TestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		c.log.Error("coordinator: cannot create report request", zap.Error(err))
		return
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		c.log.Error("coordinator: cannot do report request", zap.Error(err))
		return
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		c.log.Error("coordinator: cannot connect to the Internet", zap.Error(err))
		return
	}
}

func (c *Coordinator) testInternetConfig() interface{} {
	return struct {
		Config   config.Config     `json:"config"`
		Settings database.Settings `json:"settings"`
	}{
		*c.config,
		*c.database.Data.Settings,
	}
}

func (c *Coordinator) syncStats() {
	c.log.Debug("coordinator: syncing statuses...")

	c.syncXrayStats()
	c.syncServerStats()
}

func (c *Coordinator) syncXrayStats() {
	c.log.Debug("coordinator: syncing xray statuses...")

	stats, err := c.xray.QueryStats()
	if err != nil {
		c.log.Error("coordinator: cannot fetch query stats", zap.Error(err))
		return
	}

	users := map[int]int64{}
	for _, s := range stats {
		parts := strings.Split(s.GetName(), ">>>")
		if parts[0] == "user" {
			id, err := strconv.Atoi(parts[1])
			if err != nil {
				c.log.Error("coordinator: unknown user", zap.String("id", parts[1]), zap.Error(err))
				continue
			}
			users[id] += s.GetValue()
		} else if parts[0] == "outbound" && parts[1] == "shadowsocks" {
			c.database.Data.Stats.Outbound += s.GetValue()
		} else if parts[0] == "outbound" && parts[1] == "freedom" {
			c.database.Data.Stats.Freedom += s.GetValue()
		} else if parts[0] == "inbound" && parts[1] == "shadowsocks" {
			c.database.Data.Stats.Inbound += s.GetValue()
		}
	}

	isSyncRequired := false
	for _, u := range c.database.Data.Users {
		if b, found := users[u.Id]; found {
			u.UsedBytes += b
			u.Used = utils.RoundFloat(float64(u.UsedBytes)/1000/1000/1000, 2)
			if u.Quota != 0 && u.Used > float64(u.Quota) {
				u.Enabled = false
				isSyncRequired = true
			}
		}
	}

	c.database.Save()

	if isSyncRequired {
		c.log.Debug("coordinator: user syncing is required")
		c.SyncUsers()
	}
}

func (c *Coordinator) syncServerStats() {
	c.log.Debug("coordinator: syncing server statuses...")

	isSyncRequired := false
	for _, server := range c.database.Data.Servers {
		oldStatus := server.Status
		if utils.PortAvailable(server.Host, server.Port) {
			server.Status = database.ServerStatusAvailable
		} else if server.Status == database.ServerStatusAvailable {
			server.Status = database.ServerStatusUnstable
		} else {
			server.Status = database.ServerStatusUnavailable
		}
		if server.Status != oldStatus {
			isSyncRequired = true
		}
	}

	if isSyncRequired {
		c.log.Debug("coordinator: server syncing is required")
		c.database.Save()
		c.SyncServers()
	}
}

func New(c *config.Config, hc *http.Client, l *zap.Logger, d *database.Database, x *xray.Xray) *Coordinator {
	return &Coordinator{config: c, log: l, database: d, xray: x, hc: hc}
}
