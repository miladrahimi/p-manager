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
	p.initSsdOutboundPort()
	p.initSsdInboundPort()
	p.initSspPort()
	p.initReversePort()
	go p.runCore()
	p.connectGrpc()
}

func (x *Xray) initSsdOutboundPort() {
	x.config.Locker.Lock()
	defer x.config.Locker.Unlock()

	if x.config.SsdOutbound() == nil {
		return
	}

	var err error
	x.config.SsdOutbound().Settings.Servers[0].Port, err = utils.FreePort()
	if err != nil {
		x.log.Fatal("xray: portal cannot find free port for ssd", zap.Error(err))
	}
	x.saveConfig()
}

func (x *Xray) initSsdInboundPort() {
	x.config.Locker.Lock()
	defer x.config.Locker.Unlock()

	if x.config.SsdInbound() == nil {
		return
	}

	op := x.config.SsdInbound().Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.log.Fatal("xray: cannot find free port for ssd inbound", zap.Error(err))
		}
		x.log.Info("xray: updating ssd inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.config.UpdateSsdInbound(x.config.SsdInbound().Settings.Clients, np)
		x.saveConfig()
	}
}

func (x *Xray) initSspPort() {
	x.config.Locker.Lock()
	defer x.config.Locker.Unlock()

	if x.config.SspInbound() == nil {
		return
	}

	op := x.config.SspInbound().Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.log.Fatal("xray: cannot find free port for ssp inbound", zap.Error(err))
		}
		x.log.Info("xray: updating ssp inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.config.UpdateSspInbound(x.config.SspInbound().Settings.Clients, np)
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

func NewPortalXray(l *logger.Logger, configPath, binaryPath string) *Portal {
	return &Portal{
		Xray: Xray{
			log: l, config: NewPortalConfig(), binaryPath: binaryPath, configPath: configPath, locker: &sync.Mutex{},
		},
	}
}
