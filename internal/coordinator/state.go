package coordinator

import (
	"time"
)

// State represents coordinator state.
type State struct {
	xrayUpdatedAt time.Time
	sshTags       map[string]string
	sshLocalPorts map[string]int
}

func newState() *State {
	return &State{
		xrayUpdatedAt: time.Now(),
		sshTags:       map[string]string{},
		sshLocalPorts: map[string]int{},
	}
}

func (s *State) XrayUpdatedAt() time.Time {
	return s.xrayUpdatedAt
}

func (s *State) SshTag(nodeId string) (string, bool) {
	tag, ok := s.sshTags[nodeId]
	return tag, ok
}

func (s *State) SetSshTag(nodeId string, tag string) {
	s.sshTags[nodeId] = tag
}

func (s *State) RemoveSshTag(nodeId string) {
	delete(s.sshTags, nodeId)
}

func (s *State) SshTags() map[string]string {
	return s.sshTags
}

func (s *State) SshLocalPort(nodeId string) (int, bool) {
	port, ok := s.sshLocalPorts[nodeId]
	return port, ok
}

func (s *State) SetSshLocalPort(nodeId string, port int) {
	s.sshLocalPorts[nodeId] = port
}

func (s *State) RemoveSshLocalPort(nodeId string) {
	delete(s.sshLocalPorts, nodeId)
}

func (s *State) SshLocalPorts() map[string]int {
	return s.sshLocalPorts
}
