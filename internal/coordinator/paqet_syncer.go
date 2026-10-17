package coordinator

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/paqet"
	"github.com/miladrahimi/p-node/pkg/util"
	"go.uber.org/zap"
)

// paqetSyncer keeps one paqet client per node running for Relay RR2PQ. Each
// client dials the node's paqet server over raw packets and exposes a local
// SOCKS5 proxy that the manager's Xray balances accounts over.
type paqetSyncer struct {
	l       *logger.Logger
	db      *data.Store
	state   *State
	root    string
	config  *config.Config
	clients map[string]*paqet.Paqet

	networkMu sync.Mutex
	network   *paqet.Network
}

// nodePaqetInfo is a snapshot of the paqet-relevant fields of a node.
type nodePaqetInfo struct {
	id   string
	host string
}

// newPaqetSyncer creates a new paqet syncer.
func newPaqetSyncer(l *logger.Logger, db *data.Store, state *State, root string, c *config.Config) *paqetSyncer {
	return &paqetSyncer{
		l:       l,
		db:      db,
		state:   state,
		root:    root,
		config:  c,
		clients: map[string]*paqet.Paqet{},
	}
}

// syncPaqetClients starts, updates, and stops the paqet clients to match the
// nodes and settings in the database.
func (s *paqetSyncer) syncPaqetClients() error {
	var managerPort, nodePort int
	var privateKey string
	var nodes []nodePaqetInfo
	s.db.Read(func(d *data.Data) {
		managerPort = d.XraySettings.RelayRr2PqPort
		nodePort = d.XraySettings.RelayRr2PqNodePort
		privateKey = d.XraySettings.RealityPrivateKey
		nodes = make([]nodePaqetInfo, 0, len(d.Nodes))
		for _, n := range d.Nodes {
			nodes = append(nodes, nodePaqetInfo{id: n.Id, host: n.Host})
		}
	})
	enabled := managerPort > 0 && nodePort > 0

	currentNodes := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		currentNodes[n.id] = true
	}

	var errList error

	// Stop the clients of removed nodes, or all of them when the method is off.
	for nodeId, client := range s.clients {
		if enabled && currentNodes[nodeId] {
			continue
		}
		errList = errors.Join(errList, client.Stop())
		delete(s.clients, nodeId)
		s.state.RemovePaqetPort(nodeId)
	}
	if !enabled {
		return errors.WithStack(errList)
	}

	network, err := s.paqetNetwork()
	if err != nil {
		return errors.WithStack(errors.Join(errList, err))
	}

	for _, n := range nodes {
		client, exists := s.clients[n.id]
		socksPort, _ := s.state.PaqetPort(n.id)
		if !exists || socksPort <= 0 {
			if socksPort, err = util.FreePort(); err != nil {
				errList = errors.Join(errList, err)
				continue
			}
		}
		if !exists {
			client = paqet.New(s.l, config.PaqetBinaryPath(s.root), config.PaqetConfigPath(s.root, n.id), config.PaqetLogPath(s.root))
		}

		// Reconfigure is a no-op when nothing changed.
		cfg := paqet.ClientConfig(*network, socksPort, n.host, nodePort, composer.RelayRr2PqKey(n.id, privateKey))
		if err = client.Reconfigure(cfg); err != nil {
			errList = errors.Join(errList, err)
			s.state.RemovePaqetPort(n.id)
			if exists {
				delete(s.clients, n.id)
			}
			continue
		}
		s.clients[n.id] = client
		s.state.SetPaqetPort(n.id, socksPort)
	}

	s.l.Info("coordinator: finished syncing paqet clients", zap.Int("c", len(s.clients)))
	return errors.WithStack(errList)
}

// stopAll stops every paqet client.
func (s *paqetSyncer) stopAll() error {
	var errList error
	for nodeId, client := range s.clients {
		errList = errors.Join(errList, client.Stop())
		delete(s.clients, nodeId)
		s.state.RemovePaqetPort(nodeId)
	}
	return errors.WithStack(errList)
}

// paqetNetwork returns the network paqet uses: the configured override, or the
// detected default route (detected once and cached).
func (s *paqetSyncer) paqetNetwork() (*paqet.Network, error) {
	s.networkMu.Lock()
	defer s.networkMu.Unlock()

	if s.network != nil {
		return s.network, nil
	}
	if o := s.config.Paqet; o.Set() {
		s.network = &paqet.Network{Interface: o.Interface, Address: o.Address, RouterMac: o.RouterMac}
		return s.network, nil
	}
	network, err := paqet.DetectNetwork()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	s.l.Info("coordinator: paqet network detected", zap.String("network", network.String()))
	s.network = network
	return network, nil
}
