package xray

import (
	"github.com/miladrahimi/xray-manager/pkg/logger"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"go.uber.org/zap"
	"sync"
)

type Portal struct {
	Xray
}

func (p *Portal) Run() {
	p.initConfig()
	p.initApiPort()
	p.initShadowsocksPort()
	p.initReversePort()
	go p.runCore()
	p.connectGrpc()
}

func (x *Xray) initShadowsocksPort() {
	x.config.Locker.Lock()
	defer x.config.Locker.Unlock()

	op := x.config.ShadowsocksInbound().Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.log.Fatal("xray: cannot find free port for shadowsocks inbound", zap.Error(err))
		}
		x.log.Info("xray: updating shadowsocks inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.config.UpdateShadowsocksInbound(x.config.ShadowsocksInbound().Settings.Clients, np)
		x.saveConfig()
	}
}

func (x *Xray) initReversePort() {
	x.config.Locker.Lock()
	defer x.config.Locker.Unlock()

	op := x.config.ReverseInbound().Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.log.Fatal("xray: cannot find free port for reverse inbound", zap.Error(err))
		}
		x.log.Info("xray: updating reverse inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.config.UpdateReverseInbound(np, x.config.ReverseInbound().Settings.Password)
		x.saveConfig()
	}
}

func NewPortalXray(l *logger.Logger, configPath, binaryPath string) *Xray {
	return &Xray{
		log: l, config: NewPortalConfig(), binaryPath: binaryPath, configPath: configPath, locker: &sync.Mutex{},
	}
}
