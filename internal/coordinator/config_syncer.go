package coordinator

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/http/client"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
	"go.uber.org/zap"
)

// configSyncer is a syncer that syncs the Xray config of the nodes in the database.
type configSyncer struct {
	l        *logger.Logger
	db       *data.Store
	hc       *client.Client
	xray     *xray.Xray
	composer *composer.Composer
	state    *State
}

// newConfigSyncer creates a new config syncer.
func newConfigSyncer(
	l *logger.Logger,
	db *data.Store,
	hc *client.Client,
	xray *xray.Xray,
	composer *composer.Composer,
	state *State,
) *configSyncer {
	return &configSyncer{
		l:        l,
		db:       db,
		hc:       hc,
		xray:     xray,
		composer: composer,
		state:    state,
	}
}

// updateLocalConfig updates the local xray config.
func (c *configSyncer) updateLocalConfig() error {
	c.l.Info("coordinator: updating local configs...")

	localConfig, err := c.composer.ManagerConfig(c.state.SshConfigsByNode())
	if err != nil {
		return err
	}

	if err = c.xray.Reconfigure(localConfig); err != nil {
		return errors.WithStack(err)
	}

	c.state.SetXrayUpdatedAt(time.Now())

	return nil
}

// pushConfigToNodes pushes the config to all nodes.
func (c *configSyncer) pushConfigToNodes() {
	c.l.Info("coordinator: pushing config to all nodes...")
	var nodes []*data.Node
	c.db.Read(func(d *data.Data) {
		for _, n := range d.Nodes {
			if n.PushEnabled {
				nodes = append(nodes, n)
			}
		}
	})
	for _, n := range nodes {
		go c.pushConfigToNode(n)
	}
}

// pushConfigToStaleNodes pushes the config to stale nodes.
func (c *configSyncer) pushConfigToStaleNodes() {
	c.l.Info("coordinator: pushing config to stale nodes...")
	var nodes []*data.Node
	c.db.Read(func(d *data.Data) {
		for _, n := range d.Nodes {
			if !n.PushEnabled {
				continue
			}
			if n.PushStatus == data.NodeStatusUnavailable || n.PushStatus == data.NodeStatusProcessing {
				nodes = append(nodes, n)
			}
		}
	})
	for _, n := range nodes {
		go c.pushConfigToNode(n)
	}
}

// pushConfigToNode pushes the config to a single node.
func (c *configSyncer) pushConfigToNode(n *data.Node) {
	// Snapshot the request inputs under the read lock; the network push runs
	// without holding the store lock.
	var url, token, proxy string
	c.db.Read(func(d *data.Data) {
		url = fmt.Sprintf("%s://%s:%d/xray/config", "http", n.Host, n.HttpPort)
		token = n.HttpToken
		proxy = d.MainSettings.SingetServer
	})
	nc := c.composer.NodeConfig(n, c.state.XrayUpdatedAt())

	proxied := false
	success := false

	_, err := c.hc.Do(http.MethodPost, url, token, nc)
	if err == nil {
		success = true
	} else if proxy != "" {
		proxied = true
		_, err = c.hc.DoThrough(proxy, http.MethodPost, url, token, nc)
		if err == nil {
			success = true
		}
	}

	writeErr := c.db.Write(func(d *data.Data) {
		if success {
			n.PushedAt = time.Now().UnixMilli()
			if proxied {
				n.PushStatus = data.NodeStatusDirty
			} else {
				n.PushStatus = data.NodeStatusAvailable
			}
		} else {
			n.PushStatus = data.NodeStatusUnavailable
		}
	})

	if success {
		c.l.Debug(
			"coordinator: config pushed to a node successfully",
			zap.String("url", url),
			zap.String("proxy", proxy),
			zap.Bool("proxied", proxied),
		)
	} else {
		c.l.Error(
			"coordinator: cannot push config to a node",
			zap.String("url", url),
			zap.String("proxy", proxy),
			zap.Bool("proxied", proxied),
			zap.Error(err),
		)
	}

	if writeErr != nil {
		c.l.Error("coordinator:", zap.Error(errors.WithStack(writeErr)))
	}
}
