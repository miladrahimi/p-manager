package coordinator

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/internal/worker"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/http/client"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
	"go.uber.org/zap"
)

// Coordinator is the main app component which coordinates the synchronization of the local and remote configs.
type Coordinator struct {
	l         *logger.Logger
	db        *database.Database[data.Data]
	hc        *client.Client
	xray      *xray.Xray
	composer  *composer.Composer
	sshPool   *ssh.Pool
	sshClient *ssh.Client
	state     *State

	sshSyncer    *sshSyncer
	configSyncer *configSyncer
	statsSyncer  *statsSyncer
}

// New creates a new coordinator.
func New(
	hc *client.Client,
	logger *logger.Logger,
	db *database.Database[data.Data],
	xray *xray.Xray,
	writer *composer.Composer,
	sshManager *ssh.Pool,
	sshClient *ssh.Client,
) *Coordinator {
	c := &Coordinator{
		l:         logger,
		hc:        hc,
		db:        db,
		xray:      xray,
		composer:  writer,
		sshPool:   sshManager,
		sshClient: sshClient,
		state:     newState(),
	}

	c.sshSyncer = newSshSyncer(logger, db, sshManager, sshClient, c.state)
	c.configSyncer = newConfigSyncer(logger, db, hc, xray, writer, c.state)
	c.statsSyncer = newStatsSyncer(logger, db, hc, xray, c.UpdateConfigs)

	return c
}

// Run starts the coordinator and its workers.
func (c *Coordinator) Run(ctx context.Context) error {
	c.l.Info("coordinator: running...")

	if err := c.initialize(); err != nil {
		return errors.WithStack(err)
	}

	c.UpdateConfigs()

	go worker.New("syncSshProxies", time.Second*10, c.l, func() {
		if err := c.sshSyncer.syncSshProxies(); err != nil {
			c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
		}
	}).Start(ctx)

	go worker.New("pushConfigToStaleNodes", time.Second*10, c.l, func() {
		c.configSyncer.pushConfigToStaleNodes()
	}).Start(ctx)

	go worker.New("loadLocalStats", time.Minute, c.l, func() {
		if err := c.statsSyncer.loadLocalStats(); err != nil {
			c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
		}
	}).Start(ctx)

	go worker.New("pullStatsFromNodes", time.Minute, c.l, func() {
		c.statsSyncer.pullStatsFromNodes()
	}).Start(ctx)

	go worker.New("updateNodeSshStatus", time.Second*10, c.l, func() {
		if err := c.sshSyncer.checkSshStatuses(); err != nil {
			c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
		}
	}).Start(ctx)

	go worker.New("Backup", time.Hour, c.l, func() {
		if err := c.db.Backup(); err != nil {
			c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
		}
	}).Start(ctx)

	go worker.New("resetUsageForUsers", time.Hour, c.l, func() {
		if err := c.statsSyncer.resetUsageForUsers(); err != nil {
			c.l.Error("coordinator:", zap.Error(errors.WithStack(err)))
		}
	}).Start(ctx)

	return nil
}

// initialize initializes the coordinator.
func (c *Coordinator) initialize() (err error) {
	d := c.db.Data()

	if d.XraySettings.RrPrivateKey == "" || d.XraySettings.RrPublicKey == "" {
		d.XraySettings.RrPrivateKey, d.XraySettings.RrPublicKey, err = c.xray.GenerateX25519()
		if err != nil {
			return errors.WithStack(err)
		}
	}

	if d.XraySettings.NodeSni == "" {
		d.XraySettings.NodeSni = config.DefaultNodeSni
	}
	if d.XraySettings.ManagerSni == "" {
		d.XraySettings.ManagerSni = config.DefaultManagerSni
	}

	return errors.WithStack(c.db.Save())
}

// UpdateConfigs updates the local and node configs.
func (c *Coordinator) UpdateConfigs() {
	c.l.Info("coordinator: updating configs...")
	if err := c.sshSyncer.syncSshProxies(); err != nil {
		c.l.Error("coordinator: cannot sync ssh proxies", zap.Error(err))
	}
	if err := c.configSyncer.updateLocalConfig(); err != nil {
		c.l.Fatal("coordinator:", zap.Error(errors.WithStack(err)))
	}
	c.configSyncer.pushConfigToNodes()
}

// CheckSshStatus checks SSH connectivity for a single node.
func (c *Coordinator) CheckSshStatus(nodeId string) {
	if nodeId == "" {
		return
	}

	var node *data.Node
	for _, n := range c.db.Data().Nodes {
		if n.Id == nodeId {
			node = n
			break
		}
	}
	if node == nil {
		return
	}

	if err := c.sshSyncer.checkSshStatus(node); err != nil {
		c.l.Debug("coordinator: ssh check failed", zap.String("id", nodeId), zap.Error(err))
	}
}

// State returns the current state of the coordinator.
func (c *Coordinator) State() *State {
	return c.state
}
