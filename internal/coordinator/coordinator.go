package coordinator

import (
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/database"
	"github.com/miladrahimi/p-manager/pkg/enigma"
	"github.com/miladrahimi/p-manager/pkg/fetcher"
	"github.com/miladrahimi/p-manager/pkg/logger"
	"github.com/miladrahimi/p-manager/pkg/utils"
	"github.com/miladrahimi/p-manager/pkg/xray"
	"go.uber.org/zap"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Coordinator struct {
	config   *config.Config
	database *database.Database
	l        *logger.Logger
	fetcher  *fetcher.Fetcher
	xray     *xray.Xray
	enigma   *enigma.Enigma
	licensed bool
}

func (c *Coordinator) Run() {
	c.l.Info("coordinator: running...")

	c.SyncConfigs()

	statsWorker := time.NewTicker(time.Duration(c.config.Worker.Interval) * time.Second)
	go func() {
		for {
			<-statsWorker.C
			c.l.Info("coordinator: working...")
			c.SyncStats()
		}
	}()

	backupWorker := time.NewTicker(time.Hour)
	go func() {
		for {
			<-backupWorker.C
			c.l.Info("coordinator: backing up...")
			c.database.Backup()
		}
	}()

	go c.validateLicense()
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
	c.l.Info("coordinator: syncing configs...")
	c.syncLocalConfigs()
	c.syncRemoteConfigs()
}

func (c *Coordinator) syncLocalConfigs() {
	c.l.Info("coordinator: syncing local configs...")

	clients := c.generateShadowsocksClients()
	apiPort, err := utils.FreePort()
	if err != nil {
		c.l.Fatal("coordinator: cannot find free port for xray api", zap.Error(err))
	}

	xc := xray.NewConfig()
	c.xray.SetConfig(xc)

	xc.FindInbound("api").Port = apiPort

	if len(clients) > 0 {
		if c.database.Data.Settings.SsRelayPort > 0 {
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"relay",
				utils.Key32(),
				config.ShadowsocksMethod,
				c.database.Data.Settings.SsRelayPort,
				clients,
			))
		}
		if c.database.Data.Settings.SsReversePort > 0 {
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"reverse",
				utils.Key32(),
				config.ShadowsocksMethod,
				c.database.Data.Settings.SsReversePort,
				clients,
			))
		}
		if c.database.Data.Settings.SsDirectPort > 0 {
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"direct",
				utils.Key32(),
				config.ShadowsocksMethod,
				c.database.Data.Settings.SsDirectPort,
				clients,
			))
		}
	}

	if len(clients) > 0 {
		if c.database.Data.Settings.SsDirectPort > 0 {
			xc.Routing.Settings.Rules = append(xc.Routing.Settings.Rules, &xray.Rule{
				InboundTag:  []string{"direct"},
				OutboundTag: "freedom",
				Type:        "field",
			})
		}
		if len(c.database.Data.Servers) > 0 {
			if c.database.Data.Settings.SsRelayPort > 0 {
				xc.Routing.Settings.Rules = append(xc.Routing.Settings.Rules, &xray.Rule{
					InboundTag:  []string{"relay"},
					BalancerTag: "relay",
					Type:        "field",
				})
			}
			if c.database.Data.Settings.SsReversePort > 0 {
				xc.Routing.Settings.Rules = append(xc.Routing.Settings.Rules, &xray.Rule{
					InboundTag:  []string{"reverse"},
					BalancerTag: "portal",
					Type:        "field",
				})
			}
		}
	}

	if len(c.database.Data.Servers) > 0 {
		if c.database.Data.Settings.SsRelayPort > 0 {
			xc.Routing.Balancers = append(xc.Routing.Balancers, &xray.Balancer{Tag: "relay", Selector: []string{}})
		}
		if c.database.Data.Settings.SsReversePort > 0 {
			xc.Routing.Balancers = append(xc.Routing.Balancers, &xray.Balancer{Tag: "portal", Selector: []string{}})
		}
	}

	for _, s := range c.database.Data.Servers {
		inboundPort, err := utils.FreePort()
		if err != nil {
			c.l.Fatal("coordinator: cannot find free port for foreign inbound", zap.Error(err))
		}

		if c.database.Data.Settings.SsReversePort > 0 {
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				fmt.Sprintf("foreign-%d", s.Id),
				utils.Key32(),
				config.Shadowsocks2022Method,
				inboundPort,
				nil,
			))
			xc.Reverse.Portals = append(xc.Reverse.Portals, &xray.ReverseItem{
				Tag:    fmt.Sprintf("portal-%d", s.Id),
				Domain: fmt.Sprintf("s%d.google.com", s.Id),
			})
			xc.Routing.Settings.Rules = append(xc.Routing.Settings.Rules, &xray.Rule{
				InboundTag:  []string{fmt.Sprintf("foreign-%d", s.Id)},
				OutboundTag: fmt.Sprintf("portal-%d", s.Id),
				Type:        "field",
			})
			xc.FindBalancer("portal").Selector = append(
				xc.FindBalancer("portal").Selector,
				fmt.Sprintf("portal-%d", s.Id),
			)
		}

		if c.database.Data.Settings.SsRelayPort > 0 {
			outboundRelayPort, err := utils.FreePort()
			if err != nil {
				c.l.Fatal("coordinator: cannot find free port for relay outbound", zap.Error(err))
			}
			xc.Outbounds = append(xc.Outbounds, xc.MakeShadowsocksOutbound(
				fmt.Sprintf("relay-%d", s.Id),
				s.Host,
				utils.Key32(),
				config.Shadowsocks2022Method,
				outboundRelayPort,
			))
			xc.FindBalancer("relay").Selector = append(
				xc.FindBalancer("relay").Selector,
				fmt.Sprintf("relay-%d", s.Id),
			)
		}
	}

	c.xray.Restart()
}

func (c *Coordinator) syncRemoteConfigs() {
	c.l.Info("coordinator: syncing remote configs...")

	for _, s := range c.database.Data.Servers {
		xc := xray.NewConfig()

		if c.database.Data.Settings.SsRelayPort > 0 {
			relayOutbound := c.xray.Config().FindOutbound(fmt.Sprintf("relay-%d", s.Id))
			xc.Inbounds = append(xc.Inbounds, xc.MakeShadowsocksInbound(
				"direct",
				relayOutbound.Settings.Servers[0].Password,
				relayOutbound.Settings.Servers[0].Method,
				relayOutbound.Settings.Servers[0].Port,
				nil,
			))
			xc.Routing.Settings.Rules = append(
				xc.Routing.Settings.Rules,
				&xray.Rule{
					Type:        "field",
					InboundTag:  []string{"direct"},
					OutboundTag: "freedom",
				},
			)
		}

		if c.database.Data.Settings.SsReversePort > 0 {
			foreignOutbound := c.xray.Config().FindInbound(fmt.Sprintf("foreign-%d", s.Id))
			xc.Outbounds = append(xc.Outbounds, xc.MakeShadowsocksOutbound(
				"foreign",
				c.database.Data.Settings.Host,
				foreignOutbound.Settings.Password,
				foreignOutbound.Settings.Method,
				foreignOutbound.Port,
			))
			xc.Reverse.Bridges = append(xc.Reverse.Bridges, &xray.ReverseItem{
				Tag:    "bridge",
				Domain: fmt.Sprintf("s%d.google.com", s.Id),
			})
			xc.Routing.Settings.Rules = append(
				xc.Routing.Settings.Rules,
				&xray.Rule{
					Type:        "field",
					InboundTag:  []string{"bridge"},
					Domain:      []string{fmt.Sprintf("full:s%d.google.com", s.Id)},
					OutboundTag: "foreign",
				},
				&xray.Rule{
					Type:        "field",
					InboundTag:  []string{"bridge"},
					OutboundTag: "freedom",
				},
			)
		}

		go c.updateRemoteConfigs(s, xc)
	}
}

func (c *Coordinator) updateRemoteConfigs(s *database.Server, xc *xray.Config) {
	url := fmt.Sprintf("%s://%s:%d/v1/configs", "http", s.Host, s.HttpPort)
	c.l.Info("coordinator: updating remote configs...", zap.String("url", url))

	_, err := c.fetcher.Do(http.MethodPost, url, xc, map[string]string{
		echo.HeaderContentType:   echo.MIMEApplicationJSON,
		echo.HeaderAuthorization: fmt.Sprintf("Bearer %s", s.HttpToken),
		"X-App-Name":             config.AppName,
		"X-App-AppVersion":       config.AppVersion,
	})
	if err != nil {
		c.l.Error("coordinator: cannot update remote configs", zap.Error(err))
		s.Status = database.ServerStatusUnavailable
	} else {
		s.Status = database.ServerStatusAvailable
	}
}

func (c *Coordinator) SyncStats() {
	c.l.Info("coordinator: syncing stats...")

	c.database.Locker.Lock()
	defer c.database.Locker.Unlock()

	servers := map[string]int64{}
	users := map[string]int64{}

	sts := c.xray.QueryStats()
	for _, qs := range sts {
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

	isSyncConfigsRequired := false
	for _, u := range c.database.Data.Users {
		if bytes, found := users[strconv.Itoa(u.Id)]; found {
			u.UsedBytes += bytes
			u.Used = utils.RoundFloat(float64(u.UsedBytes)/1000/1000/1000, 2)
			if u.Quota > 0 && u.Used > u.Quota {
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

func (c *Coordinator) validateLicense() {
	url := "https://x.miladrahimi.com/p-manager/v1/servers"
	body := map[string]interface{}{
		"host": c.database.Data.Settings.Host,
		"port": c.config.HttpServer.Port,
	}
	headers := map[string]string{
		echo.HeaderContentType: echo.MIMEApplicationJSON,
		"X-App-Name":           config.AppName,
		"X-App-Version":        config.AppVersion,
	}
	if r, err := c.fetcher.Do(http.MethodPost, url, body, headers); err != nil {
		c.l.Warn("coordinator: remote license failed", zap.Error(err))
	} else {
		var response map[string]string
		if err = json.Unmarshal(r, &response); err != nil {
			c.l.Warn("coordinator: cannot unmarshall license response", zap.Error(err))
		}
		if license, found := response["license"]; found {
			if err = os.WriteFile(config.LicensePath, []byte(license), 0755); err != nil {
				c.l.Warn("coordinator: cannot write license file", zap.Error(err))
			}
		} else {
			c.l.Warn("coordinator: no remote license found")
		}
	}

	if !utils.FileExist(config.LicensePath) {
		c.licensed = false
		c.l.Info("coordinator: no license file found")
	} else {
		if err := c.enigma.Init(); err != nil {
			c.l.Warn("coordinator: cannot init enigma", zap.Error(err))
		}
		licenseFile, err := os.ReadFile(config.LicensePath)
		if err != nil {
			c.l.Warn("coordinator: cannot open license file", zap.Error(err))
		} else {
			key := fmt.Sprintf("%s:%d", c.database.Data.Settings.Host, c.config.HttpServer.Port)
			c.licensed = c.enigma.Verify(key, string(licenseFile))
			c.l.Info("coordinator: license file checked", zap.Bool("valid", c.licensed))
		}
	}
}

func (c *Coordinator) Licensed() bool {
	return c.licensed
}

func New(
	c *config.Config,
	f *fetcher.Fetcher,
	l *logger.Logger,
	d *database.Database,
	x *xray.Xray,
	e *enigma.Enigma,
) *Coordinator {
	return &Coordinator{config: c, l: l, database: d, xray: x, fetcher: f, enigma: e}
}
