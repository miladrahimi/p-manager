package ssh

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-node/pkg/logger"
)

// Pool represents a processes of SSH SOCKS proxies.
type Pool struct {
	l          *logger.Logger
	stdoutPath string
	stderrPath string
	locker     sync.Mutex
	processes  map[string]*Process
}

// New creates a new SSH proxy manager.
func New(l *logger.Logger, stdoutPath, stderrPath string) *Pool {
	return &Pool{
		l:          l,
		stdoutPath: stdoutPath,
		stderrPath: stderrPath,
		processes:  map[string]*Process{},
	}
}

// Start starts a proxy for the given tag and config if it doesn't already exist.
func (m *Pool) Start(tag string, config *Config) error {
	if tag == "" {
		return errors.New("ssh: tag is empty")
	}
	if config == nil {
		return errors.New("ssh: config is nil")
	}
	if err := config.Validate(); err != nil {
		return errors.WithStack(err)
	}

	m.locker.Lock()
	if existing := m.processes[tag]; existing != nil {
		m.locker.Unlock()
		return nil
	}

	proxy, err := Start(m.l, config, m.stdoutPath, m.stderrPath)
	if err != nil {
		m.locker.Unlock()
		return errors.WithStack(err)
	}

	m.processes[tag] = proxy
	m.locker.Unlock()
	return nil
}

// Stop stops and deletes the started proxy for the given tag.
func (m *Pool) Stop(tag string) error {
	if tag == "" {
		return errors.New("ssh: tag is empty")
	}

	m.locker.Lock()
	existing := m.processes[tag]
	delete(m.processes, tag)
	m.locker.Unlock()

	if existing == nil {
		return nil
	}

	return errors.WithStack(existing.Stop())
}

// StopAll stops and removes all proxies.
func (m *Pool) StopAll() error {
	m.locker.Lock()
	entries := make([]*Process, 0, len(m.processes))
	for tag, entry := range m.processes {
		delete(m.processes, tag)
		entries = append(entries, entry)
	}
	m.locker.Unlock()

	var err error
	for _, proxy := range entries {
		err = errors.Join(err, proxy.Stop())
	}
	return errors.WithStack(err)
}
