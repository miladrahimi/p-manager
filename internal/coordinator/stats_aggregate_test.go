package coordinator

import (
	"testing"

	"github.com/xtls/xray-core/app/stats/command"
)

func stat(name string, value int64) *command.Stat {
	return &command.Stat{Name: name, Value: value}
}

func TestAggregateNodeStats(t *testing.T) {
	stats := []*command.Stat{
		stat("user>>>acc-1>>>traffic>>>uplink", 100),
		stat("user>>>acc-1>>>traffic>>>downlink", 900),
		stat("user>>>relay>>>traffic>>>downlink", 300),
		stat("inbound>>>remote-rr>>>traffic>>>uplink", 400),
		stat("inbound>>>remote-rr>>>traffic>>>downlink", 1000),
		stat("inbound>>>remote-cxh>>>traffic>>>downlink", 2000),
		stat("inbound>>>relay-rr2rr>>>traffic>>>downlink", 5000), // counted on the manager
		stat("inbound>>>api>>>traffic>>>downlink", 7),
		stat("outbound>>>out>>>traffic>>>downlink", 9999),
		stat("garbage", 1),
	}

	// Distinct ports: every client inbound byte is the node's.
	got := aggregateNodeStats(stats, "")
	if got.users["acc-1"] != 1000 || got.users["relay"] != 300 {
		t.Fatalf("users: %v", got.users)
	}
	if got.node != 3400 || got.methods["remote-rr"] != 1400 || got.methods["remote-cxh"] != 2000 || len(got.methods) != 2 {
		t.Fatalf("node %d, methods %v", got.node, got.methods)
	}

	// Shared port: the relay client's bytes rode the remote-rr inbound and are
	// already counted on the manager.
	got = aggregateNodeStats(stats, "relay")
	if got.node != 3100 || got.methods["remote-rr"] != 1100 || got.methods["remote-cxh"] != 2000 {
		t.Fatalf("shared port: node %d, methods %v", got.node, got.methods)
	}
}

func TestAggregateLocalStats(t *testing.T) {
	stats := []*command.Stat{
		stat("user>>>acc-1>>>traffic>>>downlink", 800),
		stat("user>>>bridge-1>>>traffic>>>downlink", 600), // node-1's reverse bridge
		stat("inbound>>>direct-rr>>>traffic>>>downlink", 100),
		stat("inbound>>>relay-rr2rr>>>traffic>>>downlink", 200),
		stat("inbound>>>relay-rr2ssh>>>traffic>>>downlink", 300),
		stat("inbound>>>relay-rr2pq>>>traffic>>>downlink", 400),
		stat("inbound>>>reverse-rr>>>traffic>>>downlink", 500),
		stat("inbound>>>reverse-rr-tunnel>>>traffic>>>downlink", 500), // same bytes again, not counted
		stat("outbound>>>relay-rr2rr-node-1>>>traffic>>>downlink", 50),
		stat("outbound>>>relay-rr2ssh-node-1-1>>>traffic>>>downlink", 60),
		stat("outbound>>>relay-rr2pq-node-2>>>traffic>>>downlink", 70),
		stat("outbound>>>out>>>traffic>>>downlink", 9999),
	}
	got := aggregateLocalStats(stats, map[string]string{"bridge-1": "node-1"})

	if got.users["acc-1"] != 800 || len(got.users) != 1 {
		t.Fatalf("users (bridge must not be one): %v", got.users)
	}
	if got.nodes["node-1"] != 50+600 || got.nodes["node-1-1"] != 60 || got.nodes["node-2"] != 70 {
		t.Fatalf("nodes: %v", got.nodes)
	}
	if got.total != 1500 {
		t.Fatalf("total %d, want 1500 (reverse-rr counted once)", got.total)
	}
	want := map[string]int64{"direct-rr": 100, "relay-rr2rr": 200, "relay-rr2ssh": 300, "relay-rr2pq": 400, "reverse-rr": 500}
	for method, bytes := range want {
		if got.methods[method] != bytes {
			t.Errorf("method %s: %d, want %d", method, got.methods[method], bytes)
		}
	}
	if len(got.methods) != len(want) {
		t.Errorf("methods: %v", got.methods)
	}
}
