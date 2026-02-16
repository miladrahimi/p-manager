package coordinator

import (
	"context"
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
		for nodeId := range s.state.SshConfigs() {
			s.state.RemoveSshConfig(nodeId)
		}
		return errors.WithStack(s.pool.StopAll())
	}

	// Get current nodes from database.
	currentNodes := make(map[string]*data.Node)
	for _, node := range s.db.Data().Nodes {
		currentNodes[node.Id] = node
	}

	// Stop all ssh proxies that are not belong to current nodes.
	for nodeId := range s.state.SshConfigs() {
		if _, exists := currentNodes[nodeId]; !exists {
			s.state.RemoveSshConfig(nodeId)
			errList = errors.Join(errList, s.pool.Stop(nodeId))
		}
	}

	// Start/Update ssh proxies for current nodes.
	for _, node := range currentNodes {
		proxyConfig, hasConfig := s.state.SshConfig(node.Id)

		if hasConfig && proxyConfig != nil && proxyConfig.Connection != nil {
			// Skip node if it doesn't require update.
			if proxyConfig.Connection.Host == node.Host &&
				proxyConfig.Connection.User == node.SshUser &&
				proxyConfig.Connection.Port == node.SshPort {
				continue
			}

			// Stop node if it requires update.
			err := s.pool.Stop(node.Id)
			errList = errors.Join(errList, err)
		}

		// Find a free port for the new/updated node.
		freePort, err := util.FreePort()
		if err != nil {
			errList = errors.Join(errList, err)
			continue
		}

		// Start ssh proxies for the new/updated node.
		connectionConfig := ssh.NewConnectionConfig(node.Host, node.SshUser, node.SshPort)
		newProxyConfig := ssh.NewProxyConfig(connectionConfig, freePort)
		if err = s.pool.Start(node.Id, newProxyConfig); err != nil {
			errList = errors.Join(errList, err)
		} else {
			s.state.SetSshConfig(node.Id, newProxyConfig)
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
