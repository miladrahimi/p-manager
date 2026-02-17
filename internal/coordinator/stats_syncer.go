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
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/http/client"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
	"github.com/xtls/xray-core/app/stats/command"
	"go.uber.org/zap"
)

// statsSyncer is a syncer that syncs the stats of the nodes in the database.
type statsSyncer struct {
	l             *logger.Logger
	db            *database.Database[data.Data]
	hc            *client.Client
	xray          *xray.Xray
	updateConfigs func()
}

// newStatsSyncer creates a new stats syncer.
func newStatsSyncer(
	l *logger.Logger,
	db *database.Database[data.Data],
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
	if s.db.Data().XraySettings.RemoteRrPort == 0 {
		return
	}

	s.l.Info("coordinator: pulling stats from all nodes...")
	for _, node := range s.db.Data().Nodes {
		go s.pullStatsFromNode(node)
	}
}

// pullStatsFromNode pulls the stats of a single node.
func (s *statsSyncer) pullStatsFromNode(node *data.Node) {
	url := fmt.Sprintf("%s://%s:%d/xray/stats", "http", node.Host, node.HttpPort)

	s.l.Info("coordinator: pulling stats from a node...", zap.String("url", url))

	response, err := s.hc.Do(http.MethodGet, url, node.HttpToken, nil)
	if err != nil {
		s.l.Error("cannot fetch node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
		return
	}

	var queryStats []*command.Stat
	if err = json.Unmarshal(response, &queryStats); err != nil {
		s.l.Error("cannot read remote node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
		return
	}

	parseTrafficStat := func(name string) (string, string, bool) {
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
			if tag == "remote-rr" || tag == "relay-rr2rr" {
				nodeUsageBytes += value
			}
		}
	}

	shouldSync := false
	db := s.db.Data()
	for _, u := range db.Users {
		if bytes, found := users[u.ProxyId]; found {
			u.UsageBytes = util.SafeSumI64(u.UsageBytes, bytes)
			u.Usage = util.Bytes2GB(u.UsageBytes)
			if u.Quota > 0 && u.Usage > u.Quota {
				u.Enabled = false
				shouldSync = true
				s.l.Debug("coordinator: user disabled", zap.String("id", u.Id))
			}
		}
	}
	if shouldSync && s.updateConfigs != nil {
		go s.updateConfigs()
	}

	node.UsageBytes = util.SafeSumI64(node.UsageBytes, nodeUsageBytes)
	node.Usage = util.Bytes2GB(node.UsageBytes)

	db.Stats.TotalUsageBytes = util.SafeSumI64(db.Stats.TotalUsageBytes, nodeUsageBytes)
	db.Stats.TotalUsage = util.Bytes2GB(db.Stats.TotalUsageBytes)

	if err = s.db.Save(); err != nil {
		s.l.Error("cannot save remote node stats", zap.String("url", url), zap.Error(errors.WithStack(err)))
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

	parseTrafficStat := func(name string) (string, string, bool) {
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
			if strings.HasPrefix(tag, "relay-rr2rr-") {
				nodes[strings.TrimPrefix(tag, "relay-rr2rr-")] += value
			} else if strings.HasPrefix(tag, "relay-rr2ssh-") {
				nodes[strings.TrimPrefix(tag, "relay-rr2ssh-")] += value
			}
		case "inbound":
			switch tag {
			case "direct-rr", "relay-rr2rr", "relay-rr2ssh":
				totalBytes += value
			}
		}
	}

	db := s.db.Data()
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

	shouldSync := false
	for _, u := range db.Users {
		if bytes, found := users[u.ProxyId]; found {
			u.UsageBytes = util.SafeSumI64(u.UsageBytes, bytes)
			u.Usage = util.Bytes2GB(u.UsageBytes)
			if u.Quota > 0 && u.Usage > u.Quota {
				u.Enabled = false
				shouldSync = true
				s.l.Debug("coordinator: user disabled", zap.String("id", u.Id))
			}
		}
	}
	if shouldSync && s.updateConfigs != nil {
		go s.updateConfigs()
	}

	err = s.db.Save()
	return errors.WithStack(err)
}

// resetUsageForUsers resets the usages of all users.
func (s *statsSyncer) resetUsageForUsers() error {
	if s.db.Data().MainSettings.ResetPolicy != "monthly" {
		return nil
	}

	s.l.Info("coordinator: resetting usage for all users...")

	for _, u := range s.db.Data().Users {
		if time.Unix(u.UsageResetAt, 0).Format("2006-01") == time.Now().Format("2006-01") {
			continue
		}

		u.Usage = 0
		u.UsageBytes = 0
		u.Enabled = true
		u.UsageResetAt = time.Now().Unix()
	}

	if err := s.db.Save(); err != nil {
		return errors.WithStack(err)
	}

	if s.updateConfigs != nil {
		go s.updateConfigs()
	}

	return nil
}
