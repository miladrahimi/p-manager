package provisioner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/miladrahimi/p-manager/pkg/hetzner"
)

// fakeApi serves canned Hetzner list responses (single page each).
func fakeApi(t *testing.T, responses map[string]any) *hetzner.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.Error(w, `{"error":{"code":"not_found","message":"no"}}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(server.Close)
	return hetzner.New("test").WithBaseUrl(server.URL)
}

func serverType(name, arch string, deprecated bool, cores int, memory float64, price string, available ...string) map[string]any {
	prices := []map[string]any{}
	locations := []map[string]any{}
	for _, loc := range []string{"fsn1", "nbg1", "hel1", "ash"} {
		prices = append(prices, map[string]any{"location": loc, "price_monthly": map[string]any{"gross": price}})
		isAvailable := false
		for _, a := range available {
			if a == loc {
				isAvailable = true
			}
		}
		locations = append(locations, map[string]any{"name": loc, "available": isAvailable})
	}
	return map[string]any{
		"name": name, "architecture": arch, "deprecated": deprecated, "cores": cores, "memory": memory,
		"prices": prices, "locations": locations,
	}
}

var locations = map[string]any{"locations": []map[string]any{
	{"name": "fsn1", "country": "DE"}, {"name": "nbg1", "country": "DE"},
	{"name": "hel1", "country": "FI"}, {"name": "ash", "country": "US"},
}}

func TestCheapestServerTypePicksCheapestInStockX86InGermany(t *testing.T) {
	client := fakeApi(t, map[string]any{
		"/locations": locations,
		"/server_types": map[string]any{"server_types": []map[string]any{
			serverType("cx23", "x86", false, 2, 4, "6.64"),                   // cheapest but out of stock
			serverType("cax11", "arm", false, 2, 4, "7.24", "fsn1", "nbg1"),  // arm
			serverType("cpx11", "x86", false, 2, 2, "6.64", "ash"),           // only outside Germany
			serverType("cpx02", "x86", false, 1, 1, "7.24", "nbg1", "hel1"),  // expected
			serverType("cpx12", "x86", false, 1, 2, "13.90", "fsn1", "nbg1"), // pricier
			serverType("old", "x86", true, 1, 1, "1.00", "fsn1", "nbg1"),     // deprecated
		}},
	})

	name, location, err := cheapestServerType(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if name != "cpx02" || location != "nbg1" {
		t.Fatalf("got %s in %s, want cpx02 in nbg1", name, location)
	}
}

func TestCheapestServerTypePrefersMoreMemoryOnTie(t *testing.T) {
	client := fakeApi(t, map[string]any{
		"/locations": locations,
		"/server_types": map[string]any{"server_types": []map[string]any{
			serverType("cpx11", "x86", false, 2, 2, "6.64", "fsn1"),
			serverType("cx23", "x86", false, 2, 4, "6.64", "fsn1"),
		}},
	})

	name, _, err := cheapestServerType(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if name != "cx23" {
		t.Fatalf("got %s, want cx23", name)
	}
}

func TestCheapestServerTypeFailsWhenNothingInStock(t *testing.T) {
	client := fakeApi(t, map[string]any{
		"/locations": locations,
		"/server_types": map[string]any{"server_types": []map[string]any{
			serverType("cpx11", "x86", false, 2, 2, "6.64", "ash"),
		}},
	})

	if _, _, err := cheapestServerType(context.Background(), client); err == nil {
		t.Fatal("expected an error")
	}
}

func TestLatestImagePicksNewestDebian(t *testing.T) {
	client := fakeApi(t, map[string]any{
		"/images": map[string]any{"images": []map[string]any{
			{"name": "ubuntu-26.04", "os_version": "26.04"},
			{"name": "debian-12", "os_version": "12"},
			{"name": "debian-14", "os_version": "14", "deprecated": "2026-01-01T00:00:00Z"},
			{"name": "debian-13", "os_version": "13"},
		}},
	})

	image, err := latestImage(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if image != "debian-13" {
		t.Fatalf("got %s, want debian-13", image)
	}
}

func TestServerDetails(t *testing.T) {
	var server hetzner.Server
	raw := `{"id":7,"name":"p-node-x","created":"2026-10-09T17:25:16+00:00",
		"public_net":{"ipv4":{"ip":"1.2.3.4"}},
		"server_type":{"name":"cx23","cores":2,"memory":4,"disk":40,
			"prices":[{"location":"fsn1","price_monthly":{"net":"5.49","gross":"6.6429"},"price_hourly":{"net":"0.0088","gross":"0.0106"}}]},
		"location":{"name":"fsn1","city":"Falkenstein","country":"DE"},
		"image":{"name":"debian-13"}}`
	if err := json.Unmarshal([]byte(raw), &server); err != nil {
		t.Fatal(err)
	}

	details := serverDetails(&server)
	want := map[string]string{
		"Server":   "p-node-x (#7)",
		"IP":       "1.2.3.4",
		"Type":     "cx23",
		"Specs":    "2 vCPU, 4 GB RAM, 40 GB disk",
		"Location": "fsn1 (Falkenstein, DE)",
		"Image":    "debian-13",
		"Price":    "€6.64/month gross (€0.0088/hour net)",
	}
	for k, v := range want {
		if details[k] != v {
			t.Errorf("%s: got %q, want %q", k, details[k], v)
		}
	}
}
