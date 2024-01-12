package coordinator

import (
	"go.uber.org/zap"
	"net/http"
	"shadowsocks-manager/internal/config"
	"shadowsocks-manager/internal/database"
	"shadowsocks-manager/internal/utils"
	"shadowsocks-manager/internal/xray"
	"strconv"
	"strings"
	"time"
)

type Coordinator struct {
	http     *http.Client
	config   *config.Config
	database *database.Database
	log      *zap.Logger
	xray     *xray.Xray
}

func (c *Coordinator) Run() {
	go func() {
		c.log.Debug("coordinator starting...")
		for {
			c.log.Debug("coordinator working...")
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
			Method:   "chacha20-ietf-poly1305",
		})
	}

	c.xray.UpdateClients(clients)
	c.syncXrayStatuses()
}

func (c *Coordinator) SyncServers() {
	c.log.Debug("coordinator: syncing servers...")

	servers := make([]xray.Server, 0, len(c.database.Data.Servers))
	for _, n := range c.database.Data.Servers {
		if n.Status == database.ServerStatusProcessing || n.Status == database.ServerStatusUnavailable {
			continue
		}
		servers = append(servers, xray.Server{
			Address:  n.Host,
			Port:     n.Port,
			Method:   "chacha20-ietf-poly1305",
			Password: n.Password,
		})
	}
	c.xray.UpdateServers(servers)
}

func (c *Coordinator) SyncServersAndStatuses() {
	c.log.Debug("coordinator: syncing servers and updating statuses...")

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
				c.log.Error("coordinator: cannot detect user", zap.String("id", parts[1]), zap.Error(err))
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

	isDirty := false
	for _, u := range c.database.Data.Users {
		if bytes, found := users[u.Id]; found {
			u.UsedBytes += bytes
			u.Used = utils.RoundFloat(float64(u.UsedBytes)/1024/1024/1024, 2)
			if u.Quota != 0 && u.Used > float64(u.Quota) {
				u.Enabled = false
				isDirty = true
			}
		}
	}

	c.database.Save()

	if isDirty {
		c.log.Debug("coordinator: user syncing required")
		c.SyncUsers()
	}
}

func (c *Coordinator) syncServerStatuses() {
	c.log.Debug("coordinator: syncing server statuses...")

	isDirty := false
	for _, server := range c.database.Data.Servers {
		oldStatus := server.Status
		if utils.IsPortAvailable(server.Host, server.Port) {
			server.Status = database.ServerStatusAvailable
		} else if server.Status != database.ServerStatusUnavailable {
			if server.Status == database.ServerStatusUnstable {
				server.Status = database.ServerStatusUnavailable
			} else {
				server.Status = database.ServerStatusUnstable
			}
		}
		if server.Status != oldStatus {
			isDirty = true
		}
	}

	if isDirty {
		c.log.Debug("coordinator: server syncing required")
		c.database.Save()
		c.SyncServers()
	}
}

func New(c *config.Config, l *zap.Logger, h *http.Client, d *database.Database, x *xray.Xray) *Coordinator {
	return &Coordinator{config: c, log: l, http: h, database: d, xray: x}
}
