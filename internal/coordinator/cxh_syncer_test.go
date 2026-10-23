package coordinator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/database"
	"github.com/miladrahimi/p-node/pkg/logger"
)

// fakeCloudflare answers the few Cloudflare endpoints the syncer uses and
// remembers the DNS records it holds.
type fakeCloudflare struct {
	mu          sync.Mutex
	records     map[string]map[string]any // id -> record
	nextId      int
	revoked     []string
	certsIssued int
}

func newFakeCloudflare(t *testing.T) (*fakeCloudflare, *httptest.Server) {
	f := &fakeCloudflare{records: map[string]map[string]any{}}
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, result any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "errors": []any{}, "result": result})
	}
	mux.HandleFunc("GET /zones", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") != "example.com" {
			ok(w, []any{})
			return
		}
		ok(w, []any{map[string]any{"id": "zone-1", "name": "example.com"}})
	})
	mux.HandleFunc("POST /certificates", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body["csr"].(string), "CERTIFICATE REQUEST") {
			t.Errorf("certificate request without a CSR: %v", body)
		}
		f.mu.Lock()
		f.certsIssued++
		f.mu.Unlock()
		ok(w, map[string]any{"id": "cert-1", "certificate": "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"})
	})
	mux.HandleFunc("DELETE /certificates/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.revoked = append(f.revoked, r.PathValue("id"))
		f.mu.Unlock()
		ok(w, map[string]any{"id": r.PathValue("id")})
	})
	mux.HandleFunc("POST /zones/{zone}/dns_records", func(w http.ResponseWriter, r *http.Request) {
		var record map[string]any
		_ = json.NewDecoder(r.Body).Decode(&record)
		f.mu.Lock()
		f.nextId++
		id := fmt.Sprintf("rec-%03d", f.nextId)
		record["id"] = id
		f.records[id] = record
		f.mu.Unlock()
		ok(w, record)
	})
	mux.HandleFunc("DELETE /zones/{zone}/dns_records/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		_, found := f.records[r.PathValue("id")]
		delete(f.records, r.PathValue("id"))
		f.mu.Unlock()
		if !found {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []any{map[string]any{"code": 81044, "message": "Record does not exist."}}})
			return
		}
		ok(w, map[string]any{"id": r.PathValue("id")})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return f, server
}

func (f *fakeCloudflare) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.records)
}

func newTestCdnSyncer(t *testing.T, baseUrl string, onCertificate func()) (*cdnSyncer, *data.Store) {
	t.Helper()
	d := data.Default()
	db, err := database.New(t.TempDir(), d)
	if err != nil {
		t.Fatal(err)
	}
	d.Nodes = []*data.Node{{Id: "node-1", Host: "10.0.0.1"}, {Id: "node-2", Host: "node2.example.org"}}
	d.XraySettings.RemoteCxhPort = 2053
	d.MainSettings.CloudflareToken = "token"
	d.MainSettings.CdnDomain = "example.com"
	d.MainSettings.CdnRotationHours = 6
	store := data.NewStore(db)
	// The logger writes under ./storage/logs; run from a scratch directory.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "storage", "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	l, err := logger.New("debug", "2006-01-02 15:04:05.000", make(chan struct{}))
	if err != nil {
		t.Fatal(err)
	}
	return newCdnSyncer(l, store, baseUrl, onCertificate), store
}

func TestCdnSyncerIssuesCertificateAndRecords(t *testing.T) {
	fake, server := newFakeCloudflare(t)
	pushed := 0
	s, store := newTestCdnSyncer(t, server.URL, func() { pushed++ })

	if err := s.syncCdn(); err != nil {
		t.Fatal(err)
	}

	var cdn data.Cdn
	store.Read(func(d *data.Data) { cdn = *d.Cdn })
	if !cdn.Ready() || cdn.ZoneId != "zone-1" || cdn.CertificateId != "cert-1" {
		t.Fatalf("cdn state not ready: %+v", cdn)
	}
	if !strings.HasPrefix(cdn.CxhPath, "/") || len(cdn.CxhPath) != 17 {
		t.Fatalf("unexpected path %q", cdn.CxhPath)
	}
	if pushed != 1 {
		t.Fatalf("want one push after the certificate, got %d", pushed)
	}
	if fake.count() != 2 || len(cdn.Records) != 2 {
		t.Fatalf("want one record per node, got %d at cloudflare, %d stored", fake.count(), len(cdn.Records))
	}
	for _, r := range cdn.Records {
		if !strings.HasSuffix(r.Name, ".example.com") {
			t.Errorf("record %q not under the domain", r.Name)
		}
		got := fake.records[r.Id]
		wantType := "A"
		if r.NodeId == "node-2" {
			wantType = "CNAME"
		}
		if got["type"] != wantType || got["proxied"] != true || got["content"] != r.Content {
			t.Errorf("record %s: %v", r.Id, got)
		}
	}

	// A second run with nothing changed is a no-op.
	if err := s.syncCdn(); err != nil {
		t.Fatal(err)
	}
	if fake.count() != 2 || fake.certsIssued != 1 || pushed != 1 {
		t.Fatalf("second run changed things: %d records, %d certs, %d pushes", fake.count(), fake.certsIssued, pushed)
	}
}

func TestCdnSyncerRotatesAndKeepsPrevious(t *testing.T) {
	fake, server := newFakeCloudflare(t)
	s, store := newTestCdnSyncer(t, server.URL, nil)
	if err := s.syncCdn(); err != nil {
		t.Fatal(err)
	}

	// Age the records past the interval: each node gets a new one and keeps the old.
	age := func() {
		_ = store.Write(func(d *data.Data) {
			for _, r := range d.Cdn.Records {
				r.CreatedAt = time.Now().Add(-7 * time.Hour).UnixMilli()
			}
		})
	}
	age()
	if err := s.syncCdn(); err != nil {
		t.Fatal(err)
	}
	var cdn data.Cdn
	store.Read(func(d *data.Data) { cdn = *d.Cdn })
	if len(cdn.NodeRecords("node-1")) != 2 || fake.count() != 4 {
		t.Fatalf("want 2 records per node after rotation, got %d stored for node-1, %d at cloudflare", len(cdn.NodeRecords("node-1")), fake.count())
	}

	// Another rotation drops the oldest, so a node never has more than two.
	age()
	if err := s.syncCdn(); err != nil {
		t.Fatal(err)
	}
	store.Read(func(d *data.Data) { cdn = *d.Cdn })
	if len(cdn.NodeRecords("node-1")) != 2 || fake.count() != 4 {
		t.Fatalf("want still 2 records per node, got %d stored for node-1, %d at cloudflare", len(cdn.NodeRecords("node-1")), fake.count())
	}
}

func TestCdnSyncerRemovesGoneAndChangedNodes(t *testing.T) {
	fake, server := newFakeCloudflare(t)
	s, store := newTestCdnSyncer(t, server.URL, nil)
	if err := s.syncCdn(); err != nil {
		t.Fatal(err)
	}

	_ = store.Write(func(d *data.Data) {
		d.Nodes = d.Nodes[:1]        // node-2 gone
		d.Nodes[0].Host = "10.0.0.9" // node-1 moved
	})
	if err := s.syncCdn(); err != nil {
		t.Fatal(err)
	}
	var cdn data.Cdn
	store.Read(func(d *data.Data) { cdn = *d.Cdn })
	if len(cdn.Records) != 1 || fake.count() != 1 || cdn.Records[0].Content != "10.0.0.9" {
		t.Fatalf("want one record for the moved node, got %+v (%d at cloudflare)", cdn.Records, fake.count())
	}
}

func TestCdnSyncerDisableAndDomainChange(t *testing.T) {
	fake, server := newFakeCloudflare(t)
	pushed := 0
	s, store := newTestCdnSyncer(t, server.URL, func() { pushed++ })
	if err := s.syncCdn(); err != nil {
		t.Fatal(err)
	}

	// Off: records go, the certificate stays for when it is turned on again.
	_ = store.Write(func(d *data.Data) { d.XraySettings.RemoteCxhPort = 0 })
	if err := s.syncCdn(); err != nil {
		t.Fatal(err)
	}
	var cdn data.Cdn
	store.Read(func(d *data.Data) { cdn = *d.Cdn })
	if fake.count() != 0 || len(cdn.Records) != 0 || !cdn.Ready() {
		t.Fatalf("after disabling: %d records at cloudflare, state %+v", fake.count(), cdn)
	}

	// On again with another domain: the old certificate is revoked and a new
	// one issued; the path survives so existing clients only change host.
	path := cdn.CxhPath
	_ = store.Write(func(d *data.Data) {
		d.XraySettings.RemoteCxhPort = 2053
		d.MainSettings.CdnDomain = "other.com"
	})
	err := s.syncCdn()
	if err == nil || !strings.Contains(err.Error(), "zone other.com not found") {
		t.Fatalf("want a zone lookup failure for the unknown domain, got %v", err)
	}
	store.Read(func(d *data.Data) { cdn = *d.Cdn })
	if len(fake.revoked) != 1 || fake.revoked[0] != "cert-1" || cdn.Certificate != "" || cdn.Domain != "other.com" || cdn.CxhPath != path {
		t.Fatalf("after the domain change: revoked %v, state %+v", fake.revoked, cdn)
	}
	if pushed != 2 {
		t.Fatalf("want a push for the dropped certificate, got %d pushes", pushed)
	}
}
