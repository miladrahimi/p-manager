package coordinator

import (
	"context"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/logger"
	"go.uber.org/zap"
)

// sshSyncer is a syncer that syncs the SSH status of the nodes in the database.
type sshSyncer struct {
	l      *logger.Logger
	db     *database.Database[data.Data]
	pool   *ssh.Pool
	client *ssh.Client
	state  *State
}

// newSshSyncer creates a new SSH syncer.
func newSshSyncer(
	l *logger.Logger,
	db *database.Database[data.Data],
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

	// Stop All if RelayRr2SshPort is disabled.
	if s.db.Data().XraySettings.RelayRr2SshPort <= 0 {
		for nodeId := range s.state.SshConfigsByNode() {
			s.state.RemoveSshConfigs(nodeId)
		}
		return errors.WithStack(s.pool.StopAll())
	}

	// Get current nodes from database.
	currentNodes := make(map[string]*data.Node)
	for _, node := range s.db.Data().Nodes {
		currentNodes[node.Id] = node
	}

	// Stop all ssh proxies that are not belong to current nodes.
	for nodeId, configs := range s.state.SshConfigsByNode() {
		if _, exists := currentNodes[nodeId]; !exists {
			s.state.RemoveSshConfigs(nodeId)
			errList = errors.Join(errList, s.stopNodeProxies(nodeId, configs))
		}
	}

	desiredConnections := s.db.Data().XraySettings.RelayRr2SshConnections
	if desiredConnections < 1 {
		desiredConnections = 1
	}

	// Start/Update ssh proxies for current nodes.
	for _, node := range currentNodes {
		proxyConfigs, hasConfig := s.state.SshConfigs(node.Id)

		requiresReset := !hasConfig || len(proxyConfigs) != desiredConnections
		if !requiresReset {
			for _, proxyConfig := range proxyConfigs {
				if proxyConfig == nil || proxyConfig.Connection == nil {
					requiresReset = true
					break
				}
				if proxyConfig.Connection.Host != node.Host ||
					proxyConfig.Connection.User != node.SshUser ||
					proxyConfig.Connection.Port != node.SshPort {
					requiresReset = true
					break
				}
			}
		}

		if !requiresReset {
			continue
		}

		if hasConfig {
			errList = errors.Join(errList, s.stopNodeProxies(node.Id, proxyConfigs))
		}

		connectionConfig := ssh.NewConnectionConfig(node.Host, node.SshUser, node.SshPort)
		newConfigs := make([]*ssh.ProxyConfig, 0, desiredConnections)

		for i := 0; i < desiredConnections; i++ {
			freePort, err := util.FreePort()
			if err != nil {
				errList = errors.Join(errList, err)
				continue
			}

			newProxyConfig := ssh.NewProxyConfig(connectionConfig, freePort)
			if err = s.pool.Start(s.makeSshProxyTag(node.Id, i), newProxyConfig); err != nil {
				errList = errors.Join(errList, err)
				continue
			}
			newConfigs = append(newConfigs, newProxyConfig)
		}

		if len(newConfigs) == 0 {
			s.state.RemoveSshConfigs(node.Id)
		} else {
			s.state.SetSshConfigs(node.Id, newConfigs)
		}
	}

	s.l.Info("coordinator: finished syncing ssh proxies", zap.Int("c", len(s.state.sshConfigsByNode)))
	return errors.WithStack(errList)
}

// checkSshStatuses checks SSH connectivity for all nodes.
func (s *sshSyncer) checkSshStatuses() error {
	if s.client == nil {
		return errors.New("ssh: client is nil")
	}

	changed := false
	for _, node := range s.db.Data().Nodes {
		nodeChanged, err := s.updateNodeSshStatus(node)
		if err != nil {
			return err
		}
		if nodeChanged {
			changed = true
		}
	}

	if changed {
		return errors.WithStack(s.db.Save())
	}
	return nil
}

// checkSshStatus checks SSH connectivity for a single node.
func (s *sshSyncer) checkSshStatus(node *data.Node) error {
	if s.client == nil {
		return errors.New("ssh: client is nil")
	}
	if node == nil {
		return errors.New("ssh: node is nil")
	}

	previous := node.SshStatus
	changed, err := s.updateNodeSshStatus(node)
	if err != nil {
		return err
	}
	if changed && node.SshStatus != previous {
		return errors.WithStack(s.db.Save())
	}
	return nil
}

// updateNodeSshStatus updates the SSH status of a single node.
func (s *sshSyncer) updateNodeSshStatus(node *data.Node) (bool, error) {
	config := ssh.NewConnectionConfig(node.Host, node.SshUser, node.SshPort)
	var status data.NodeStatus = data.NodeStatusAvailable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := s.client.Check(ctx, config); err != nil {
		status = data.NodeStatusUnavailable
		s.l.Debug("coordinator: ssh check failed", zap.String("host", node.Host), zap.Error(err))
	}
	cancel()
	if node.SshStatus == status {
		return false, nil
	}
	node.SshStatus = status
	return true, nil
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
