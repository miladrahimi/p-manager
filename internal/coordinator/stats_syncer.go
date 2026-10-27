package coordinator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/miladrahimi/p-manager/internal/composer"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-manager/pkg/util"
	"github.com/miladrahimi/p-node/pkg/http/client"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
	"github.com/xtls/xray-core/app/stats/command"
	"go.uber.org/zap"
)

// statsSyncer is a syncer that syncs the stats of the nodes in the database.
type statsSyncer struct {
	l             *logger.Logger
	db            *data.Store
	hc            *client.Client
	xray          *xray.Xray
	updateConfigs func()
}

// newStatsSyncer creates a new stats syncer.
func newStatsSyncer(
	l *logger.Logger,
	db *data.Store,
	hc *client.Client,
	xray *xray.Xray,
	updateConfigs func(),
) *statsSyncer {
	return &statsSyncer{
		l:             l,
		db:            db,
		hc:            hc,
		xray:          xray,
		updateConfigs: updateConfigs,
	}
}

// nodeStatsEnabled reports whether nodes carry account traffic the manager does
// not see itself: the client inbounds Remote RR and Remote CXH. Relayed and
// reversed traffic is already counted on the manager.
func nodeStatsEnabled(xs *data.XraySettings) bool {
	return xs.RemoteRrPort != 0 || xs.RemoteCxhPort != 0
}

// pullStatsBeforePush pulls a node's stats right before a config push, since
// P-Node restarts Xray on a changed config and that zeroes its counters.
// Unreachable nodes are skipped: the stale-node retry would only add noise.
func (s *statsSyncer) pullStatsBeforePush(node *data.Node) {
	var enabled bool
	s.db.Read(func(d *data.Data) {
		enabled = nodeStatsEnabled(d.XraySettings) && node.PushStatus == data.NodeStatusAvailable
	})
	if enabled {
		s.pullStatsFromNode(node)
	}
}

// pullStatsFromNodes pulls the stats of all nodes.
func (s *statsSyncer) pullStatsFromNodes() {
	var enabled bool
	var nodes []*data.Node
	s.db.Read(func(d *data.Data) {
		enabled = nodeStatsEnabled(d.XraySettings)
		if enabled {
			nodes = append(nodes, d.Nodes...)
		}
	})
	if !enabled {
		return
	}

	s.l.Info("coordinator: pulling stats from all nodes...")
	for _, node := range nodes {
		go s.pullStatsFromNode(node)
	}
}

// pullStatsFromNode pulls the stats of a single node.
func (s *statsSyncer) pullStatsFromNode(node *data.Node) {
	var url, token, relayClientId string
	s.db.Read(func(d *data.Data) {
		url = fmt.Sprintf("%s://%s:%d/xray/stats", "http", node.Host, node.HttpPort)
		token = node.HttpToken
		if d.XraySettings.NodeRrShared() {
			relayClientId = composer.RelayRr2RrClientId(node.Id, d.XraySettings.RealityPrivateKey)
		}
	})

	s.l.Info("coordinator: pulling stats from a node...", zap.String("url", url))

	response, err := s.hc.Do(http.MethodGet, url, token, nil)
	if err != nil {
		s.l.Error("cannot fetch node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
		return
	}

	var queryStats []*command.Stat
	if err = json.Unmarshal(response, &queryStats); err != nil {
		s.l.Error("cannot read remote node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
		return
	}

	traffic := aggregateNodeStats(queryStats, relayClientId)
	users, nodeUsageBytes := traffic.users, traffic.node

	shouldSync := false
	err = s.db.Write(func(db *data.Data) {
		for _, u := range db.Accounts {
			if bytes, found := users[u.ProxyId]; found {
				u.UsageBytes = util.SafeSumI64(u.UsageBytes, bytes)
				u.Usage = util.Bytes2GB(u.UsageBytes)
				if u.Quota > 0 && u.Usage > u.Quota {
					u.Enabled = false
					shouldSync = true
					s.l.Debug("coordinator: account disabled", zap.String("id", u.Id))
				}
			}
		}

		if n := db.FindNodeById(node.Id); n != nil {
			n.UsageBytes = util.SafeSumI64(n.UsageBytes, nodeUsageBytes)
			n.Usage = util.Bytes2GB(n.UsageBytes)

			db.Stats.TotalUsageBytes = util.SafeSumI64(db.Stats.TotalUsageBytes, nodeUsageBytes)
			db.Stats.TotalUsage = util.Bytes2GB(db.Stats.TotalUsageBytes)
			for method, bytes := range traffic.methods {
				db.Stats.AddMethodUsage(method, bytes)
			}
		}
	})
	if err != nil {
		s.l.Error("cannot save remote node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
	}

	if shouldSync && s.updateConfigs != nil {
		go s.updateConfigs()
	}
}

// loadLocalStats loads the local XraySettings instance stats.
func (s *statsSyncer) loadLocalStats() error {
	s.l.Info("coordinator: loading local stats...")

	queryStats, err := s.xray.QueryStats()
	if err != nil {
		return errors.WithStack(err)
	}
	if payload, err := json.Marshal(queryStats); err != nil {
		s.l.Debug("coordinator: cannot marshal local stats", zap.Error(errors.WithStack(err)))
	} else {
		s.l.Debug("coordinator: local stats response", zap.String("stats", string(payload)))
	}

	// Reverse bridges show up as users of the reverse tunnel; map them to nodes.
	bridgeIdToNode := map[string]string{}
	s.db.Read(func(d *data.Data) {
		for _, n := range d.Nodes {
			bridgeIdToNode[composer.ReverseRrBridgeId(n.Id, d.XraySettings.RealityPrivateKey)] = n.Id
		}
	})
	traffic := aggregateLocalStats(queryStats, bridgeIdToNode)
	nodes, users, totalBytes := traffic.nodes, traffic.users, traffic.total

	shouldSync := false
	err = s.db.Write(func(db *data.Data) {
		if totalBytes > 0 {
			db.Stats.TotalUsageBytes = util.SafeSumI64(db.Stats.TotalUsageBytes, totalBytes)
		}
		for method, bytes := range traffic.methods {
			db.Stats.AddMethodUsage(method, bytes)
		}

		for _, n := range db.Nodes {
			if bytes, found := nodes[n.Id]; found {
				n.UsageBytes = util.SafeSumI64(n.UsageBytes, bytes)
			}
			n.Usage = util.Bytes2GB(n.UsageBytes)
		}

		db.Stats.TotalUsage = util.Bytes2GB(db.Stats.TotalUsageBytes)

		for _, u := range db.Accounts {
			if bytes, found := users[u.ProxyId]; found {
				u.UsageBytes = util.SafeSumI64(u.UsageBytes, bytes)
				u.Usage = util.Bytes2GB(u.UsageBytes)
				if u.Quota > 0 && u.Usage > u.Quota {
					u.Enabled = false
					shouldSync = true
					s.l.Debug("coordinator: account disabled", zap.String("id", u.Id))
				}
			}
		}
	})
	if err != nil {
		return errors.WithStack(err)
	}

	if shouldSync && s.updateConfigs != nil {
		go s.updateConfigs()
	}

	return nil
}

// resetUsageForAccounts resets the usages of all accounts.
func (s *statsSyncer) resetUsageForAccounts() error {
	var policyMonthly bool
	s.db.Read(func(d *data.Data) {
		policyMonthly = d.MainSettings.ResetPolicy == "monthly"
	})
	if !policyMonthly {
		return nil
	}

	s.l.Info("coordinator: resetting usage for all accounts...")

	reset := 0
	if err := s.db.Write(func(d *data.Data) {
		now := time.Now()
		for _, u := range d.Accounts {
			if time.Unix(u.UsageResetAt, 0).Format("2006-01") == now.Format("2006-01") {
				continue
			}

			u.Usage = 0
			u.UsageBytes = 0
			u.Enabled = true
			u.UsageResetAt = now.Unix()
			reset++
		}
	}); err != nil {
		return errors.WithStack(err)
	}

	// Only a real reset changes the configs. This worker runs hourly, and an
	// unconditional update pushed every node a config with a fresh timestamp
	// each hour (older P-Nodes restarted Xray on it, dropping every user).
	if reset > 0 && s.updateConfigs != nil {
		go s.updateConfigs()
	}

	return nil
}
