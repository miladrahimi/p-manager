package coordinator

import (
	"context"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/util"
	"go.uber.org/zap"
)

// sshSyncer is a syncer that syncs the SSH status of the nodes in the database.
type sshSyncer struct {
	l      *logger.Logger
	db     *data.Store
	pool   *ssh.Pool
	client *ssh.Client
	state  *State
}

// nodeSshInfo is a snapshot of the SSH-relevant fields of a node, captured
// under the store read lock so the rest of the sync can run lock-free.
type nodeSshInfo struct {
	id      string
	host    string
	sshUser string
	sshPort int
}

// newSshSyncer creates a new SSH syncer.
func newSshSyncer(
	l *logger.Logger,
	db *data.Store,
	pool *ssh.Pool,
	client *ssh.Client,
	state *State,
) *sshSyncer {
	return &sshSyncer{
		l:      l,
		db:     db,
		pool:   pool,
		client: client,
		state:  state,
	}
}

// syncSshProxies syncs the SSH SOCKS proxies for the nodes in the database.
func (s *sshSyncer) syncSshProxies() error {
	var errList error

	// Snapshot the SSH-relevant state under the read lock so the (slower)
	// pool operations below run without holding the store lock.
	var relayPort, desiredConnections int
	var nodes []nodeSshInfo
	s.db.Read(func(d *data.Data) {
		relayPort = d.XraySettings.RelayRr2SshPort
		desiredConnections = d.XraySettings.RelayRr2SshConnections
		nodes = make([]nodeSshInfo, 0, len(d.Nodes))
		for _, node := range d.Nodes {
			if !node.SshEnabled {
				continue
			}
			nodes = append(nodes, nodeSshInfo{
				id:      node.Id,
				host:    node.Host,
				sshUser: node.SshUser,
				sshPort: node.SshPort,
			})
		}
	})

	// Stop All if RelayRr2SshPort is disabled.
	if relayPort <= 0 {
		for nodeId := range s.state.SshConfigsByNode() {
			s.state.RemoveSshConfigs(nodeId)
		}
		return errors.WithStack(s.pool.StopAll())
	}

	// Get current nodes from the snapshot.
	currentNodes := make(map[string]nodeSshInfo)
	for _, node := range nodes {
		currentNodes[node.id] = node
	}

	// Stop all ssh proxies that are not belong to current nodes.
	for nodeId, configs := range s.state.SshConfigsByNode() {
		if _, exists := currentNodes[nodeId]; !exists {
			s.state.RemoveSshConfigs(nodeId)
			errList = errors.Join(errList, s.stopNodeProxies(nodeId, configs))
		}
	}

	if desiredConnections < 1 {
		desiredConnections = 1
	}

	// Start/Update ssh proxies for current nodes.
	for _, node := range nodes {
		proxyConfigs, hasConfig := s.state.SshConfigs(node.id)

		requiresReset := !hasConfig || len(proxyConfigs) != desiredConnections
		if !requiresReset {
			for _, proxyConfig := range proxyConfigs {
				if proxyConfig == nil || proxyConfig.Connection == nil {
					requiresReset = true
					break
				}
				if proxyConfig.Connection.Host != node.host ||
					proxyConfig.Connection.User != node.sshUser ||
					proxyConfig.Connection.Port != node.sshPort {
					requiresReset = true
					break
				}
			}
		}

		if !requiresReset {
			continue
		}

		if hasConfig {
			errList = errors.Join(errList, s.stopNodeProxies(node.id, proxyConfigs))
		}

		connectionConfig := ssh.NewConnectionConfig(node.host, node.sshUser, node.sshPort)
		newConfigs := make([]*ssh.ProxyConfig, 0, desiredConnections)

		for i := 0; i < desiredConnections; i++ {
			freePort, err := util.FreePort()
			if err != nil {
				errList = errors.Join(errList, err)
				continue
			}

			newProxyConfig := ssh.NewProxyConfig(connectionConfig, freePort)
			if err = s.pool.Start(s.makeSshProxyTag(node.id, len(newConfigs)), newProxyConfig); err != nil {
				errList = errors.Join(errList, err)
				continue
			}
			newConfigs = append(newConfigs, newProxyConfig)
		}

		if len(newConfigs) == 0 {
			s.state.RemoveSshConfigs(node.id)
		} else {
			s.state.SetSshConfigs(node.id, newConfigs)
		}
	}

	s.l.Info("coordinator: finished syncing ssh proxies", zap.Int("c", s.state.SshConfigsCount()))
	return errors.WithStack(errList)
}

// checkSshStatuses checks SSH connectivity for all nodes.
func (s *sshSyncer) checkSshStatuses() error {
	if s.client == nil {
		return errors.New("ssh: client is nil")
	}

	// Snapshot nodes and their connection configs under the read lock; the SSH
	// probes below run outside the lock.
	type target struct {
		node *data.Node
		cfg  *ssh.ConnectionConfig
	}
	var targets []target
	s.db.Read(func(d *data.Data) {
		targets = make([]target, 0, len(d.Nodes))
		for _, node := range d.Nodes {
			if !node.SshEnabled {
				continue
			}
			targets = append(targets, target{
				node: node,
				cfg:  ssh.NewConnectionConfig(node.Host, node.SshUser, node.SshPort),
			})
		}
	})

	statuses := make(map[*data.Node]data.NodeStatus, len(targets))
	for _, t := range targets {
		statuses[t.node] = s.probe(t.cfg)
	}

	return errors.WithStack(s.applyStatuses(statuses))
}

// checkSshStatus checks SSH connectivity for a single node.
func (s *sshSyncer) checkSshStatus(node *data.Node) error {
	if s.client == nil {
		return errors.New("ssh: client is nil")
	}
	if node == nil {
		return errors.New("ssh: node is nil")
	}
	if !node.SshEnabled {
		return nil
	}

	var cfg *ssh.ConnectionConfig
	s.db.Read(func(d *data.Data) {
		cfg = ssh.NewConnectionConfig(node.Host, node.SshUser, node.SshPort)
	})

	return errors.WithStack(s.applyStatuses(map[*data.Node]data.NodeStatus{node: s.probe(cfg)}))
}

// probe checks SOCKS feasibility and returns the resulting node status. It must
// be called without holding the store lock (it performs network I/O). The SSH
// client bounds the probe with its own timeout.
func (s *sshSyncer) probe(config *ssh.ConnectionConfig) data.NodeStatus {
	if err := s.client.Check(context.Background(), config); err != nil {
		s.l.Debug("coordinator: ssh check failed", zap.String("host", config.Host), zap.Error(err))
		return data.NodeStatusUnavailable
	}
	return data.NodeStatusAvailable
}

// applyStatuses writes the probed statuses back under the write lock and saves
// only when at least one node status changed.
func (s *sshSyncer) applyStatuses(statuses map[*data.Node]data.NodeStatus) error {
	return s.db.Mutate(func(d *data.Data) (bool, error) {
		changed := false
		for node, status := range statuses {
			if node.SshStatus != status {
				node.SshStatus = status
				changed = true
			}
		}
		return changed, nil
	})
}

// makeSshProxyTag generates a unique tag for an SSH proxy.
func (s *sshSyncer) makeSshProxyTag(nodeId string, index int) string {
	return nodeId + "-" + strconv.Itoa(index+1)
}

// stopNodeProxies stops all SSH proxies for a single node.
func (s *sshSyncer) stopNodeProxies(nodeId string, configs []*ssh.ProxyConfig) (err error) {
	for i := range configs {
		err = errors.Join(err, s.pool.Stop(s.makeSshProxyTag(nodeId, i)))
	}
	return errors.WithStack(err)
}
