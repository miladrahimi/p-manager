package coordinator

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/ssh"
	"go.uber.org/zap"
)

const (
	defaultSshUser = "root"
	defaultSshPort = 22
)

func (c *Coordinator) syncSshProxies() {
	if c.ssh == nil {
		return
	}

	if c.db.Data().XraySettings.Vrrv2SshPort <= 0 {
		c.clearSshTags()
		if err := c.ssh.StopAll(); err != nil {
			c.l.Error("coordinator: cannot stop ssh proxies", zap.Error(errors.WithStack(err)))
		}
		return
	}

	desired := make(map[string]*data.Node)
	for _, node := range c.db.Data().Nodes {
		desired[node.Id] = node
	}

	for nodeId, tag := range c.state.SshTags() {
		if _, exists := desired[nodeId]; !exists {
			c.state.RemoveSshTag(nodeId)
			c.state.RemoveSshLocalPort(nodeId)
			if err := c.ssh.Remove(tag); err != nil {
				c.l.Error("coordinator: cannot remove ssh proxy", zap.String("tag", tag), zap.Error(errors.WithStack(err)))
			}
		}
	}

	for _, node := range desired {
		tag, exists := c.state.SshTag(node.Id)
		if !exists {
			tag = fmt.Sprintf("ssh-%s", node.Id)
			c.state.SetSshTag(node.Id, tag)
		}

		config := ssh.NewConfig(node.Host, defaultSshUser, defaultSshPort, 0)
		localPort, err := c.ssh.Add(tag, config)
		if err != nil {
			c.state.RemoveSshLocalPort(node.Id)
			c.l.Error(
				"coordinator: cannot add ssh proxy",
				zap.String("tag", tag),
				zap.String("host", node.Host),
				zap.Error(errors.WithStack(err)),
			)
			continue
		}
		c.state.SetSshLocalPort(node.Id, localPort)
		c.l.Info(
			"coordinator: syncing ssh proxies",
			zap.String("tag", tag),
			zap.String("id", node.Id),
			zap.Int("localPort", localPort),
		)
	}
}

func (c *Coordinator) clearSshTags() {
	for nodeId := range c.state.SshTags() {
		c.state.RemoveSshTag(nodeId)
		c.state.RemoveSshLocalPort(nodeId)
	}
}
