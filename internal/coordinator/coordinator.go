package coordinator

import (
	"github.com/labstack/gommon/random"
	"go.uber.org/zap"
	"shadowsocks-manager/internal/config"
	"shadowsocks-manager/internal/database"
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
	xray     *xray.Xray
}

func (c *Coordinator) Run() {
	c.log.Debug("coordinator: running...")
	go func() {
		for {
			c.log.Debug("coordinator: worker running...")
			c.syncStatuses()
			time.Sleep(time.Duration(c.config.Worker.Interval) * time.Second)
		}
	}()
}

func (c *Coordinator) SyncUsers() {
	c.log.Debug("coordinator: syncing users...")

	clients := make([]xray.Client, 0, len(c.database.Data.Users))
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

func (c *Coordinator) SyncUsersAndStatuses() {
	c.log.Debug("coordinator: syncing users and statuses...")
	c.SyncUsers()
	c.syncXrayStatuses()
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

func (c *Coordinator) SyncServersAndStatuses() {
	c.log.Debug("coordinator: syncing servers and statuses...")

	c.SyncServers()
	c.syncServerStatuses()
}

func (c *Coordinator) syncStatuses() {
	c.log.Debug("coordinator: syncing statuses...")

	c.syncXrayStatuses()
	c.syncServerStatuses()
}

func (c *Coordinator) syncXrayStatuses() {
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
		if bytes, found := users[u.Id]; found {
			u.UsedBytes += bytes
			u.Used = utils.RoundFloat(float64(u.UsedBytes)/1024/1024/1024, 2)
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

func (c *Coordinator) syncServerStatuses() {
	c.log.Debug("coordinator: syncing server statuses...")

	isSyncRequired := false
	for _, server := range c.database.Data.Servers {
		oldStatus := server.Status
		if utils.PortAvailable(server.Host, server.Port) {
			server.Status = database.ServerStatusAvailable
		} else {
			if server.Status == database.ServerStatusAvailable {
				server.Status = database.ServerStatusUnstable
			} else {
				server.Status = database.ServerStatusUnavailable
			}
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

func New(c *config.Config, l *zap.Logger, d *database.Database, x *xray.Xray) *Coordinator {
	return &Coordinator{config: c, log: l, database: d, xray: x}
}
