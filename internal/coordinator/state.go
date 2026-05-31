package coordinator

import (
	"maps"
	"sync"
	"time"

	"github.com/miladrahimi/p-manager/pkg/ssh"
)

// State represents coordinator state.
type State struct {
	mu               sync.RWMutex
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.xrayUpdatedAt
}

func (s *State) SetXrayUpdatedAt(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.xrayUpdatedAt = at
}

func (s *State) SshConfigs(nodeId string) ([]*ssh.ProxyConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	configs, ok := s.sshConfigsByNode[nodeId]
	return configs, ok
}

func (s *State) SetSshConfigs(nodeId string, configs []*ssh.ProxyConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sshConfigsByNode[nodeId] = configs
}

func (s *State) RemoveSshConfigs(nodeId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sshConfigsByNode, nodeId)
}

// SshConfigsByNode returns a shallow copy of the node -> proxy configs map so
// callers can iterate it safely while the syncer mutates the live map.
func (s *State) SshConfigsByNode() map[string][]*ssh.ProxyConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]*ssh.ProxyConfig, len(s.sshConfigsByNode))
	maps.Copy(out, s.sshConfigsByNode)
	return out
}

// SshConfigsCount returns the number of nodes that currently have proxy configs.
func (s *State) SshConfigsCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sshConfigsByNode)
}
