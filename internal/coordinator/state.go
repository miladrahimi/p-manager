package coordinator

import (
	"time"

	"github.com/miladrahimi/p-manager/pkg/ssh"
)

// State represents coordinator state.
type State struct {
	xrayUpdatedAt    time.Time
	sshConfigsByNode map[string]*ssh.Config
}

func newState() *State {
	return &State{
		xrayUpdatedAt:    time.Now(),
		sshConfigsByNode: map[string]*ssh.Config{},
	}
}

func (s *State) XrayUpdatedAt() time.Time {
	return s.xrayUpdatedAt
}

func (s *State) SshConfig(nodeId string) (*ssh.Config, bool) {
	config, ok := s.sshConfigsByNode[nodeId]
	return config, ok
}

func (s *State) SetSshConfig(nodeId string, c *ssh.Config) {
	s.sshConfigsByNode[nodeId] = c
}

func (s *State) RemoveSshConfig(nodeId string) {
	delete(s.sshConfigsByNode, nodeId)
}

func (s *State) SshConfigs() map[string]*ssh.Config {
	return s.sshConfigsByNode
}
