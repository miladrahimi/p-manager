package cloudflare

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRealApi exercises the client against the real Cloudflare API: zone lookup,
// an Origin CA certificate (issued then revoked) and a DNS record (created then
// deleted). It needs CF_TOKEN (Zone Read, DNS Edit, SSL and Certificates Edit)
// and CF_DOMAIN, and is skipped without them.
func TestRealApi(t *testing.T) {
	token, domain := os.Getenv("CF_TOKEN"), os.Getenv("CF_DOMAIN")
	if token == "" || domain == "" {
		t.Skip("set CF_TOKEN and CF_DOMAIN to run against the real Cloudflare API")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New(token)

	zone, err := c.FindZone(ctx, domain)
	if err != nil {
		t.Fatal(err)
	}
	if zone.Name != domain || zone.Id == "" {
		t.Fatalf("unexpected zone %+v", zone)
	}
	if _, err := c.FindZone(ctx, "nope-"+domain); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want a not-found error for an unknown zone, got %v", err)
	}

	privateKey, csr, err := GenerateCsr(OriginHostnames(domain))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(privateKey, "-----BEGIN EC PRIVATE KEY-----") {
		t.Fatalf("unexpected key PEM: %.40s", privateKey)
	}
	certificate, err := c.CreateOriginCertificate(ctx, csr, OriginHostnames(domain), 7)
	if err != nil {
		t.Fatal(err)
	}
	if certificate.Id == "" || !strings.HasPrefix(certificate.Certificate, "-----BEGIN CERTIFICATE-----") {
		t.Fatalf("unexpected certificate %+v", certificate)
	}
	t.Cleanup(func() {
		if err := c.RevokeOriginCertificate(context.Background(), certificate.Id); err != nil && !IsNotFound(err) {
			t.Errorf("revoke: %v", err)
		}
	})

	record, err := c.CreateDnsRecord(ctx, zone.Id, DnsRecord{
		Type: "A", Name: "p-manager-test." + domain, Content: "192.0.2.1", Proxied: false, Ttl: 60,
		Comment: "p-manager client test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Id == "" || record.Name != "p-manager-test."+domain {
		t.Fatalf("unexpected record %+v", record)
	}
	if err := c.DeleteDnsRecord(ctx, zone.Id, record.Id); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteDnsRecord(ctx, zone.Id, record.Id); !IsNotFound(err) {
		t.Fatalf("want not-found on the second delete, got %v", err)
	}
}
