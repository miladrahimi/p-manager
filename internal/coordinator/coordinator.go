package coordinator

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/http/client"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/worker"
	"github.com/miladrahimi/p-node/pkg/xray"
	"go.uber.org/zap"
)

// Coordinator is the main app component which coordinates the synchronization of the local and remote configs.
type Coordinator struct {
	l         *logger.Logger
	db        *data.Store
	hc        *client.Client
	xray      *xray.Xray
	composer  *composer.Composer
	sshPool   *ssh.Pool
	sshClient *ssh.Client
	state     *State

	sshSyncer    *sshSyncer
	paqetSyncer  *paqetSyncer
	cdnSyncer    *cdnSyncer
	configSyncer *configSyncer
	statsSyncer  *statsSyncer

	// updateMu serializes UpdateConfigs and the ssh/paqet proxy syncs.
	updateMu sync.Mutex
}

// New creates a new coordinator.
func New(
	hc *client.Client,
	logger *logger.Logger,
	db *data.Store,
	xray *xray.Xray,
	writer *composer.Composer,
	sshManager *ssh.Pool,
	sshClient *ssh.Client,
	root string,
	c *config.Config,
) *Coordinator {
	cdr := &Coordinator{
		l:         logger,
		hc:        hc,
		db:        db,
		xray:      xray,
		composer:  writer,
		sshPool:   sshManager,
		sshClient: sshClient,
		state:     newState(),
	}

	cdr.sshSyncer = newSshSyncer(logger, db, sshManager, sshClient, cdr.state)
	cdr.paqetSyncer = newPaqetSyncer(logger, db, cdr.state, root, c)
	cdr.configSyncer = newConfigSyncer(logger, db, hc, xray, writer, cdr.state)
	// A new certificate or path changes the node inbounds, so push them.
	cdr.cdnSyncer = newCdnSyncer(logger, db, c.Cloudflare.BaseUrl, cdr.configSyncer.pushConfigToNodes)
	cdr.statsSyncer = newStatsSyncer(logger, db, hc, xray, cdr.UpdateConfigs)
	// Xray zeroes its counters on restart, so read them before any reconfigure
	// or node push. The local read fails harmlessly before Xray's first start.
	cdr.configSyncer.beforeReconfigure = func() {
		if err := cdr.statsSyncer.loadLocalStats(); err != nil {
			logger.Debug("coordinator: no local stats before reconfigure", zap.Error(err))
		}
	}
	cdr.configSyncer.beforePush = cdr.statsSyncer.pullStatsBeforePush

	return cdr
}

// Run starts the coordinator and its workers.
func (c *Coordinator) Run(ctx context.Context) error {
	c.l.Info("coordinator: running...")

	if err := c.initialize(); err != nil {
		return errors.WithStack(err)
	}

	c.UpdateConfigs()

	worker.New("syncSshProxies", c.l, time.Second*10, func() error {
		c.updateMu.Lock()
		defer c.updateMu.Unlock()
		return errors.WithStack(c.sshSyncer.syncSshProxies())
	}).Start(ctx)

	worker.New("syncPaqetClients", c.l, time.Second*10, func() error {
		c.updateMu.Lock()
		defer c.updateMu.Unlock()
		return errors.WithStack(c.paqetSyncer.syncPaqetClients())
	}).Start(ctx)

	worker.New("syncCdn", c.l, time.Second*20, func() error {
		return errors.WithStack(c.cdnSyncer.syncCdn())
	}).Start(ctx)

	worker.New("pushConfigToStaleNodes", c.l, time.Second*10, func() error {
		c.configSyncer.pushConfigToStaleNodes()
		return nil
	}).Start(ctx)

	worker.New("loadLocalStats", c.l, time.Minute, func() error {
		return errors.WithStack(c.statsSyncer.loadLocalStats())
	}).Start(ctx)

	worker.New("pullStatsFromNodes", c.l, time.Minute, func() error {
		c.statsSyncer.pullStatsFromNodes()
		return nil
	}).Start(ctx)

	// The SSH status only feeds the admin UI, and every check is a full SSH
	// login on the node (it was 360 an hour at 10 s); adding or editing a node
	// still checks it at once (CheckSshStatus).
	worker.New("updateNodeSshStatus", c.l, time.Minute, func() error {
		return errors.WithStack(c.sshSyncer.checkSshStatuses())
	}).Start(ctx)

	worker.New("Backup", c.l, time.Hour, func() error {
		return errors.WithStack(c.db.Backup())
	}).Start(ctx)

	worker.New("resetUsageForAccounts", c.l, time.Hour, func() error {
		return errors.WithStack(c.statsSyncer.resetUsageForAccounts())
	}).Start(ctx)

	return nil
}

// initialize initializes the coordinator.
func (c *Coordinator) initialize() error {
	// X25519 keys are generated outside the store lock (it shells out to xray).
	var privateKey, publicKey string
	var needKeys bool
	c.db.Read(func(d *data.Data) {
		needKeys = d.XraySettings.RealityPrivateKey == "" || d.XraySettings.RealityPublicKey == ""
	})
	if needKeys {
		var err error
		if privateKey, publicKey, err = c.xray.GenerateX25519(); err != nil {
			return errors.WithStack(err)
		}
	}

	return errors.WithStack(c.db.Write(func(d *data.Data) {
		if needKeys {
			d.XraySettings.RealityPrivateKey = privateKey
			d.XraySettings.RealityPublicKey = publicKey
		}
		if d.XraySettings.NodeSni == "" {
			d.XraySettings.NodeSni = config.DefaultNodeSni
		}
		if d.XraySettings.ManagerSni == "" {
			d.XraySettings.ManagerSni = config.DefaultManagerSni
		}
		// TODO: Remove this in next version
		for _, u := range d.Accounts {
			if u.ProxyId == "" {
				u.ProxyId = util.Uuid()
			}
		}
		// Backfill a per-node pull token for nodes created before it existed.
		for _, n := range d.Nodes {
			if n.PullToken == "" {
				n.PullToken = util.Uuid()
			}
		}
	}))
}

// UpdateConfigs updates the local and node configs.
func (c *Coordinator) UpdateConfigs() {
	c.updateMu.Lock()
	defer c.updateMu.Unlock()

	c.l.Info("coordinator: updating configs...")
	if err := c.sshSyncer.syncSshProxies(); err != nil {
		c.l.Error("coordinator: cannot sync ssh proxies", zap.Error(err))
	}
	if err := c.paqetSyncer.syncPaqetClients(); err != nil {
		c.l.Error("coordinator: cannot sync paqet clients", zap.Error(err))
	}
	// A reconfigure failure (e.g. a port conflict from bad settings) must not
	// crash the whole manager; log and keep serving so it can be fixed.
	if err := c.configSyncer.updateLocalConfig(); err != nil {
		c.l.Error("coordinator: cannot update local config", zap.Error(errors.WithStack(err)))
	}
	c.configSyncer.pushConfigToNodes()
}

// CheckSshStatus checks SSH connectivity for a single node.
func (c *Coordinator) CheckSshStatus(nodeId string) {
	if nodeId == "" {
		return
	}

	var node *data.Node
	c.db.Read(func(d *data.Data) {
		for _, n := range d.Nodes {
			if n.Id == nodeId {
				node = n
				break
			}
		}
	})
	if node == nil {
		return
	}

	if err := c.sshSyncer.checkSshStatus(node); err != nil {
		c.l.Debug("coordinator: ssh check failed", zap.String("id", nodeId), zap.Error(err))
	}
}

// Close stops the paqet clients.
func (c *Coordinator) Close() error {
	c.updateMu.Lock()
	defer c.updateMu.Unlock()
	return errors.WithStack(c.paqetSyncer.stopAll())
}

// State returns the current state of the coordinator.
func (c *Coordinator) State() *State {
	return c.state
}
