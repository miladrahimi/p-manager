package ssh

import (
	"context"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/logger"
)

// Manager controls SSH SOCKS proxies by tag.
type Manager struct {
	l      *logger.Logger
	ctx    context.Context
	locker sync.Mutex
	pool   map[string]*managerEntry
}

// New creates a new SSH proxy manager.
func New(ctx context.Context, l *logger.Logger) *Manager {
	return &Manager{
		l:    l,
		ctx:  ctx,
		pool: map[string]*managerEntry{},
	}
}

type managerEntry struct {
	proxy     *Ssh
	localPort int
}

// Add starts a proxy for the given tag and config if it doesn't already exist.
// It returns the local SOCKS port for the proxy.
func (m *Manager) Add(tag string, config *Config) (int, error) {
	if strings.TrimSpace(tag) == "" {
		return 0, errors.New("ssh: tag is required")
	}
	if config == nil {
		return 0, errors.New("ssh: config is nil")
	}

	m.locker.Lock()
	if existing := m.pool[tag]; existing != nil {
		localPort := existing.localPort
		m.locker.Unlock()
		return localPort, nil
	}
	m.locker.Unlock()

	if config.LocalPort == 0 {
		localPort, err := util.FreePort()
		if err != nil {
			return 0, errors.WithStack(err)
		}
		config.LocalPort = localPort
	}

	if err := config.Validate(); err != nil {
		return 0, errors.WithStack(err)
	}

	proxy := newProxy(m.ctx, m.l, config)
	m.locker.Lock()
	if current := m.pool[tag]; current != nil {
		localPort := current.localPort
		m.locker.Unlock()
		return localPort, nil
	}
	m.pool[tag] = &managerEntry{proxy: proxy, localPort: config.LocalPort}
	m.locker.Unlock()
	if err := proxy.Run(); err != nil {
		_ = m.Remove(tag)
		return 0, errors.WithStack(err)
	}
	return config.LocalPort, nil
}

// Remove stops and deletes the proxy for the given tag.
func (m *Manager) Remove(tag string) error {
	if strings.TrimSpace(tag) == "" {
		return errors.New("ssh: tag is required")
	}

	m.locker.Lock()
	existing := m.pool[tag]
	delete(m.pool, tag)
	m.locker.Unlock()

	if existing == nil {
		return nil
	}

	return errors.WithStack(existing.proxy.Stop())
}

// StopAll stops and removes all proxies.
func (m *Manager) StopAll() error {
	m.locker.Lock()
	entries := make([]*Ssh, 0, len(m.pool))
	for tag, entry := range m.pool {
		delete(m.pool, tag)
		entries = append(entries, entry.proxy)
	}
	m.locker.Unlock()

	var firstErr error
	for _, proxy := range entries {
		if err := proxy.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return errors.WithStack(firstErr)
	}

	return nil
}
