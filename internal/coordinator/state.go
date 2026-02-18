package coordinator

import (
	"time"

	"github.com/miladrahimi/p-manager/pkg/ssh"
)

// State represents coordinator state.
type State struct {
	xrayUpdatedAt    time.Time
	sshConfigsByNode map[string][]*ssh.ProxyConfig
}

func newState() *State {
	return &State{
		xrayUpdatedAt:    time.Now(),
		sshConfigsByNode: map[string][]*ssh.ProxyConfig{},
	}
}

func (s *State) XrayUpdatedAt() time.Time {
	return s.xrayUpdatedAt
}

func (s *State) SetXrayUpdatedAt(at time.Time) {
	s.xrayUpdatedAt = at
}

func (s *State) SshConfigs(nodeId string) ([]*ssh.ProxyConfig, bool) {
	configs, ok := s.sshConfigsByNode[nodeId]
	return configs, ok
}

func (s *State) SetSshConfigs(nodeId string, configs []*ssh.ProxyConfig) {
	s.sshConfigsByNode[nodeId] = configs
}

func (s *State) RemoveSshConfigs(nodeId string) {
	delete(s.sshConfigsByNode, nodeId)
}

func (s *State) SshConfigsByNode() map[string][]*ssh.ProxyConfig {
	return s.sshConfigsByNode
}
