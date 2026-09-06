package coordinator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
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

// pullStatsFromNodes pulls the stats of all nodes.
func (s *statsSyncer) pullStatsFromNodes() {
	var enabled bool
	var nodes []*data.Node
	s.db.Read(func(d *data.Data) {
		enabled = d.XraySettings.RemoteRrPort != 0
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
	var url, token string
	s.db.Read(func(d *data.Data) {
		url = fmt.Sprintf("%s://%s:%d/xray/stats", "http", node.Host, node.HttpPort)
		token = node.HttpToken
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

	users := map[string]int64{}
	var nodeUsageBytes int64

	for _, qs := range queryStats {
		kind, tag, ok := parseTrafficStat(qs.GetName())
		if !ok {
			continue
		}

		value := qs.GetValue()
		switch kind {
		case "user":
			if tag != "" {
				users[tag] += value
			}
		case "inbound":
			// Only remote-rr; relay-rr2rr is already counted by the manager's loadLocalStats (no double-count).
			if tag == "remote-rr" {
				nodeUsageBytes += value
			}
		}
	}

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

	nodes := map[string]int64{}
	users := map[string]int64{}
	var totalBytes int64

	for _, qs := range queryStats {
		kind, tag, ok := parseTrafficStat(qs.GetName())
		if !ok {
			continue
		}

		value := qs.GetValue()
		switch kind {
		case "user":
			if tag != "" {
				users[tag] += value
			}
		case "outbound":
			if rest, ok := strings.CutPrefix(tag, "relay-rr2rr-"); ok {
				nodes[rest] += value
			} else if rest, ok := strings.CutPrefix(tag, "relay-rr2ssh-"); ok {
				nodes[rest] += value
			}
		case "inbound":
			switch tag {
			case "direct-rr", "relay-rr2rr", "relay-rr2ssh":
				totalBytes += value
			}
		}
	}

	shouldSync := false
	err = s.db.Write(func(db *data.Data) {
		if totalBytes > 0 {
			db.Stats.TotalUsageBytes = util.SafeSumI64(db.Stats.TotalUsageBytes, totalBytes)
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
		}
	}); err != nil {
		return errors.WithStack(err)
	}

	if s.updateConfigs != nil {
		go s.updateConfigs()
	}

	return nil
}

// parseTrafficStat parses an Xray traffic stat name of the form
// "<kind>>>><tag>>>>traffic>>><uplink|downlink>" and returns the kind and tag.
// ok is false for stats that are not uplink/downlink traffic counters.
func parseTrafficStat(name string) (kind string, tag string, ok bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 4 {
		return "", "", false
	}
	if parts[2] != "traffic" {
		return "", "", false
	}
	if parts[3] != "uplink" && parts[3] != "downlink" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
