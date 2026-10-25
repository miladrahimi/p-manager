package composer

import (
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"

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

func TestNodeConfigRemoteCxh(t *testing.T) {
	c, node := newTestComposer(t, func(xs *data.XraySettings) {
		xs.RemoteCxhPort = 2053
	})
	// Not ready without a certificate: no inbound and no links.
	if in := inboundByTag(c.NodeConfig(node, time.Now()).Inbounds, "remote-cxh"); in != nil {
		t.Fatal("remote-cxh inbound without a certificate")
	}

	c.db.Write(func(d *data.Data) {
		d.Cdn.CxhPath = "/abc"
		d.Cdn.Domain = "example.com"
		d.Cdn.Certificate = "-----BEGIN CERTIFICATE-----\nAAA\nBBB\n-----END CERTIFICATE-----\n"
		d.Cdn.PrivateKey = "-----BEGIN EC PRIVATE KEY-----\nKKK\n-----END EC PRIVATE KEY-----\n"
		d.Cdn.Records = []*data.CdnRecord{
			{Id: "r1", NodeId: node.Id, Name: "old.example.com", Content: node.Host, CreatedAt: 1},
			{Id: "r2", NodeId: node.Id, Name: "new.example.com", Content: node.Host, CreatedAt: 2},
			{Id: "r3", NodeId: "other", Name: "x.example.com", Content: "10.0.0.2", CreatedAt: 3},
		}
	})

	xc := c.NodeConfig(node, time.Now())
	in := inboundByTag(xc.Inbounds, "remote-cxh")
	if in == nil {
		t.Fatalf("no remote-cxh inbound in %d inbounds", len(xc.Inbounds))
	}
	if err := validator.New(validator.WithRequiredStructEnabled()).Struct(in); err != nil {
		t.Fatal(err)
	}
	ss := in.StreamSettings
	if in.Port != 2053 || ss.Network != "xhttp" || ss.Security != "tls" || ss.XhttpSettings.Path != "/abc" {
		t.Fatalf("unexpected inbound: port %d, %s/%s, path %q", in.Port, ss.Network, ss.Security, ss.XhttpSettings.Path)
	}
	cert := ss.TlsSettings.Certificates[0]
	if len(cert.Certificate) != 4 || cert.Certificate[1] != "AAA" || len(cert.Key) != 3 || cert.CertificateFile != "" {
		t.Fatalf("certificate not inlined as PEM lines: %+v", cert)
	}
	if ids := clientIds(in); len(ids) != 1 || ids[0] != "proxy-1" || in.Settings.Clients[0].Flow != "" {
		t.Fatalf("want the account without a flow, got %+v", in.Settings.Clients)
	}

	var account *data.Account
	c.db.Read(func(d *data.Data) { account = d.Accounts[0] })
	// Only the newest subdomain is handed out; the previous one keeps working
	// for clients that already hold it.
	links := c.AccountLinks(account)
	if _, ok := links["remote-cxh-old.example.com@127.0.0.1"]; ok {
		t.Fatal("link for the previous subdomain")
	}
	link, ok := links["remote-cxh-new.example.com@127.0.0.1"]
	if !ok {
		t.Fatalf("no link for the newest subdomain in %v", links)
	}
	for _, want := range []string{"@new.example.com:2053", "type=xhttp", "security=tls", "path=%2Fabc", "mode=packet-up", "sni=new.example.com", "host=new.example.com", "fp=chrome", "alpn=h2"} {
		if !strings.Contains(link, want) {
			t.Errorf("link lacks %q: %s", want, link)
		}
	}
	if strings.Contains(link, "flow=") || strings.Contains(link, "pbk=") {
		t.Errorf("link carries Reality fields: %s", link)
	}
	if _, ok := links["remote-cxh-x.example.com@127.0.0.1"]; ok {
		t.Fatal("link for another node's record")
	}
}
