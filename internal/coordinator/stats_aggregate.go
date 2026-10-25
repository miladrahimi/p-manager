package coordinator

import (
	"strings"

	"github.com/xtls/xray-core/app/stats/command"
)

// nodeTraffic is what one stats pull from a node adds up to.
type nodeTraffic struct {
	users   map[string]int64 // client email (account proxy id) -> bytes
	node    int64            // bytes the node served to clients directly
	methods map[string]int64 // client inbound tag (method) -> bytes
}

// aggregateNodeStats sums a node's Xray counters. Only the client inbounds
// count towards the node and the methods: relay-rr2rr traffic is already
// counted on the manager via its relay outbound. With a shared node port the
// remote-rr inbound carries that relay traffic too, so the relay client's
// user bytes are taken out again.
func aggregateNodeStats(stats []*command.Stat, relayClientId string) nodeTraffic {
	t := nodeTraffic{users: map[string]int64{}, methods: map[string]int64{}}
	for _, qs := range stats {
		kind, tag, ok := parseTrafficStat(qs.GetName())
		if !ok {
			continue
		}
		value := qs.GetValue()
		switch kind {
		case "user":
			if tag != "" {
				t.users[tag] += value
			}
		case "inbound":
			if tag == "remote-rr" || tag == "remote-cxh" {
				t.node += value
				t.methods[tag] += value
			}
		}
	}
	if relayClientId != "" {
		relayed := t.users[relayClientId]
		t.node = max(t.node-relayed, 0)
		t.methods["remote-rr"] = max(t.methods["remote-rr"]-relayed, 0)
	}
	return t
}

// localTraffic is what one read of the manager's Xray counters adds up to.
type localTraffic struct {
	users   map[string]int64 // client email (account proxy id) -> bytes
	nodes   map[string]int64 // node id -> bytes relayed or reversed through it
	total   int64            // bytes that entered through the manager's client inbounds
	methods map[string]int64 // client inbound tag (method) -> bytes
}

// aggregateLocalStats sums the manager's Xray counters. Relayed traffic is
// attributed to nodes by the relay outbounds; reversed traffic by the bridge
// clients on the reverse tunnel (bridgeIdToNode maps their ids to node ids).
func aggregateLocalStats(stats []*command.Stat, bridgeIdToNode map[string]string) localTraffic {
	t := localTraffic{users: map[string]int64{}, nodes: map[string]int64{}, methods: map[string]int64{}}
	for _, qs := range stats {
		kind, tag, ok := parseTrafficStat(qs.GetName())
		if !ok {
			continue
		}
		value := qs.GetValue()
		switch kind {
		case "user":
			if nodeId, isBridge := bridgeIdToNode[tag]; isBridge {
				t.nodes[nodeId] += value
			} else if tag != "" {
				t.users[tag] += value
			}
		case "outbound":
			for _, prefix := range []string{"relay-rr2rr-", "relay-rr2ssh-", "relay-rr2pq-"} {
				if nodeId, ok := strings.CutPrefix(tag, prefix); ok {
					t.nodes[nodeId] += value
					break
				}
			}
		case "inbound":
			switch tag {
			case "direct-rr", "relay-rr2rr", "relay-rr2ssh", "relay-rr2pq", "reverse-rr":
				t.total += value
				t.methods[tag] += value
			}
		}
	}
	return t
}

// parseTrafficStat splits an Xray traffic counter name like
// "user>>>email>>>traffic>>>uplink" into its kind and tag.
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
