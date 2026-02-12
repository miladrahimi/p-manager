package coordinator

import (
	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"github.com/miladrahimi/p-manager/pkg/util"
	"go.uber.org/zap"
)

// syncSshProxies syncs the SSH SOCKS proxies for the nodes in the database.
func (c *Coordinator) syncSshProxies() error {
	var errList error

	// Stop All if Vrrv2SshPort is disabled.
	if c.db.Data().XraySettings.Vrrv2SshPort <= 0 {
		for nodeId := range c.state.SshConfigs() {
			c.state.RemoveSshConfig(nodeId)
		}
		return errors.WithStack(c.sshPool.StopAll())
	}

	// Get current nodes from database.
	currentNodes := make(map[string]*data.Node)
	for _, node := range c.db.Data().Nodes {
		currentNodes[node.Id] = node
	}

	// Stop all ssh proxies that are not belong to current nodes.
	for nodeId := range c.state.SshConfigs() {
		if _, exists := currentNodes[nodeId]; !exists {
			c.state.RemoveSshConfig(nodeId)
			errList = errors.Join(errList, c.sshPool.Stop(nodeId))
		}
	}

	// Start/Update ssh proxies for current nodes.
	for _, node := range currentNodes {
		sshConfig, hasConfig := c.state.SshConfig(node.Id)

		if hasConfig && sshConfig != nil {
			// Skip node if it doesn't require update.
			if sshConfig.Host == node.Host &&
				sshConfig.User == node.SshUser &&
				sshConfig.ServerPort == node.SshPort {
				continue
			}

			// Stop node if it requires update.
			err := c.sshPool.Stop(node.Id)
			errList = errors.Join(errList, err)
		}

		// Find a free port for the new/updated node.
		freePort, err := util.FreePort()
		if err != nil {
			errList = errors.Join(errList, err)
			continue
		}

		// Start ssh proxies for the new/updated node.
		sshConfig = ssh.NewConfig(node.Host, node.SshUser, node.SshPort, freePort)
		if err = c.sshPool.Start(node.Id, sshConfig); err != nil {
			errList = errors.Join(errList, err)
		} else {
			c.state.SetSshConfig(node.Id, sshConfig)
		}
	}

	c.l.Info("coordinator: finished syncing ssh proxies", zap.Int("c", len(c.state.sshConfigsByNode)))
	return errors.WithStack(errList)
}
