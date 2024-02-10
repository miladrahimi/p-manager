package xray

import (
	"github.com/miladrahimi/xray-manager/pkg/logger"
)

type Portal struct {
	Xray
}

func (p *Portal) Run() {
	p.initApiInbound()
	p.saveConfig()
	go p.runCore()
	p.connectGrpc()
}

func NewPortalXray(l *logger.Logger, configPath, binaryPath string) *Portal {
	return &Portal{Xray: *New(l, NewConfig(), configPath, binaryPath)}
}
