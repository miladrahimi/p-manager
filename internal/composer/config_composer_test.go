package composer

import (
	"testing"
	"time"

	"github.com/miladrahimi/p-manager/internal/config"
	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/xray/config/component"
)

// newTestComposer builds a composer over an in-memory default database with
// one enabled account and one node, and returns the node.
func newTestComposer(t *testing.T, xs func(*data.XraySettings)) (*Composer, *data.Node) {
	t.Helper()
	d := data.Default()
	db, err := database.New(t.TempDir(), d)
	if err != nil {
		t.Fatal(err)
	}
	node := &data.Node{Id: "node-1", Host: "10.0.0.1", HttpPort: 8080}
	d.Accounts = []*data.Account{{Id: "acc-1", ProxyId: "proxy-1", Name: "e2e", Enabled: true}}
	d.Nodes = []*data.Node{node}
	d.XraySettings.RealityPrivateKey = "private"
	xs(d.XraySettings)
	return New(&config.Config{}, data.NewStore(db), nil), node
}

func inboundByTag(inbounds []*component.Inbound, tag string) *component.Inbound {
	for _, in := range inbounds {
		if in.Tag == tag {
			return in
		}
	}
	return nil
}

func clientIds(in *component.Inbound) []string {
	ids := make([]string, 0, len(in.Settings.Clients))
	for _, c := range in.Settings.Clients {
		ids = append(ids, c.Id)
	}
	return ids
}

func TestNodeConfigDistinctPorts(t *testing.T) {
	c, node := newTestComposer(t, func(xs *data.XraySettings) {
		xs.RemoteRrPort = 443
		xs.RelayRr2RrManagerPort = 8443
		xs.RelayRr2RrNodePort = 9443
	})
	xc := c.NodeConfig(node, time.Now())

	remote := inboundByTag(xc.Inbounds, "remote-rr")
	relay := inboundByTag(xc.Inbounds, "relay-rr2rr")
	if remote == nil || relay == nil {
		t.Fatalf("want remote-rr and relay-rr2rr inbounds, got %d inbounds", len(xc.Inbounds))
	}
	if remote.Port != 443 || relay.Port != 9443 {
		t.Fatalf("ports: remote %d, relay %d", remote.Port, relay.Port)
	}
	if ids := clientIds(remote); len(ids) != 1 || ids[0] != "proxy-1" {
		t.Fatalf("remote-rr clients: %v", ids)
	}
	if ids := clientIds(relay); len(ids) != 1 || ids[0] != RelayRr2RrClientId(node.Id, "private") {
		t.Fatalf("relay-rr2rr clients: %v", ids)
	}
}

func TestNodeConfigSharedPort(t *testing.T) {
	c, node := newTestComposer(t, func(xs *data.XraySettings) {
		xs.RemoteRrPort = 443
		xs.RelayRr2RrManagerPort = 8443
		xs.RelayRr2RrNodePort = 443
	})
	xc := c.NodeConfig(node, time.Now())

	if relay := inboundByTag(xc.Inbounds, "relay-rr2rr"); relay != nil {
		t.Fatal("shared port must not add a separate relay-rr2rr inbound")
	}
	remote := inboundByTag(xc.Inbounds, "remote-rr")
	if remote == nil {
		t.Fatal("missing remote-rr inbound")
	}
	if remote.Port != 443 {
		t.Fatalf("remote-rr port: %d", remote.Port)
	}
	ids := clientIds(remote)
	if len(ids) != 2 || ids[0] != "proxy-1" || ids[1] != RelayRr2RrClientId(node.Id, "private") {
		t.Fatalf("remote-rr clients: %v", ids)
	}
	for _, cl := range remote.Settings.Clients {
		if cl.Flow != "xtls-rprx-vision" {
			t.Fatalf("client %s flow: %q", cl.Id, cl.Flow)
		}
	}
}

// With a shared port but Relay RR2RR disabled (no manager port), the node
// serves accounts only.
func TestNodeConfigSharedPortRelayDisabled(t *testing.T) {
	c, node := newTestComposer(t, func(xs *data.XraySettings) {
		xs.RemoteRrPort = 443
		xs.RelayRr2RrNodePort = 443
	})
	xc := c.NodeConfig(node, time.Now())

	if relay := inboundByTag(xc.Inbounds, "relay-rr2rr"); relay != nil {
		t.Fatal("relay-rr2rr inbound must not exist when the relay is disabled")
	}
	remote := inboundByTag(xc.Inbounds, "remote-rr")
	if remote == nil {
		t.Fatal("missing remote-rr inbound")
	}
	if ids := clientIds(remote); len(ids) != 1 || ids[0] != "proxy-1" {
		t.Fatalf("remote-rr clients: %v", ids)
	}
}
