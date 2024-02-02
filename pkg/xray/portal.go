package xray

import (
	"github.com/miladrahimi/xray-manager/pkg/logger"
	"github.com/miladrahimi/xray-manager/pkg/utils"
	"go.uber.org/zap"
)

type Portal struct {
	Xray
}

func (p *Portal) Run() {
	p.initConfig()
	p.initApiInbound()
	p.initRelayInbound()
	p.initReverseInbound()
	p.initForeignInbound()
	go p.runCore()
	p.connectGrpc()
}

func (x *Xray) initRelayInbound() {
	if x.config.RelayInbound() == nil {
		return
	}

	op := x.config.RelayInbound().Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.l.Fatal("xray: cannot find free port for relay inbound", zap.Error(err))
		}
		x.l.Info("xray: updating relay inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.config.RelayInboundUpdate(x.config.RelayInbound().Settings.Clients, np)
		x.saveConfig()
	}
}

func (x *Xray) initReverseInbound() {
	if x.config.ReverseInbound() == nil {
		return
	}

	op := x.config.ReverseInbound().Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.l.Fatal("xray: cannot find free port for reverse inbound", zap.Error(err))
		}
		x.l.Info("xray: updating reverse inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.config.ReverseInboundUpdate(x.config.ReverseInbound().Settings.Clients, np)
		x.saveConfig()
	}
}

func (x *Xray) initForeignInbound() {
	op := x.config.ForeignInbound().Port
	if !utils.PortFree(op) {
		np, err := utils.FreePort()
		if err != nil {
			x.l.Fatal("xray: cannot find free port for foreign inbound", zap.Error(err))
		}
		x.l.Info("xray: updating foreign inbound port...", zap.Int("old", op), zap.Int("new", np))
		x.config.ForeignInboundUpdate(np, x.config.ForeignInbound().Settings.Password)
		x.saveConfig()
	}
}

func NewPortalXray(l *logger.Logger, configPath, binaryPath string) *Portal {
	return &Portal{Xray: *New(l, NewPortalConfig(), configPath, binaryPath)}
}
