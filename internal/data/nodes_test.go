package data

import (
	"encoding/json"
	"testing"
)

func TestNodeAutoIpHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		disabled bool
		pulledIp string
		want     string
		changed  bool
	}{
		{"public ipv4 moves", "1.2.3.4", false, "5.6.7.8", "5.6.7.8", true},
		{"same ip", "1.2.3.4", false, "1.2.3.4", "1.2.3.4", false},
		{"ipv4-mapped same ip", "1.2.3.4", false, "::ffff:1.2.3.4", "1.2.3.4", false},
		{"ipv4-mapped new ip", "1.2.3.4", false, "::ffff:5.6.7.8", "5.6.7.8", true},
		{"disabled", "1.2.3.4", true, "5.6.7.8", "1.2.3.4", false},
		{"domain host", "node.example.com", false, "5.6.7.8", "node.example.com", false},
		{"public to loopback (proxy)", "1.2.3.4", false, "127.0.0.1", "1.2.3.4", false},
		{"public to private (proxy)", "1.2.3.4", false, "10.0.0.2", "1.2.3.4", false},
		{"ipv4 to ipv6", "1.2.3.4", false, "2a01:4f8::1", "1.2.3.4", false},
		{"public ipv6 moves", "2a01:4f8::1", false, "2a01:4f8::2", "2a01:4f8::2", true},
		{"private moves", "10.0.0.2", false, "10.0.0.3", "10.0.0.3", true},
		{"loopback moves", "127.0.0.2", false, "127.0.0.1", "127.0.0.1", true},
		{"invalid pulled ip", "1.2.3.4", false, "", "1.2.3.4", false},
		{"unspecified", "0.0.0.0", false, "0.0.0.0", "0.0.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNode("id", tt.host, "", 0, "root", 22)
			n.AutoIpEnabled = !tt.disabled
			got, changed := n.AutoIpHost(tt.pulledIp)
			if got != tt.want || changed != tt.changed {
				t.Fatalf("AutoIpHost(%q) on %q = %q, %v; want %q, %v", tt.pulledIp, tt.host, got, changed, tt.want, tt.changed)
			}
		})
	}
}

func TestNodeUnmarshalAutoIpDefault(t *testing.T) {
	var old, off Node
	if err := json.Unmarshal([]byte(`{"id":"a","host":"1.2.3.4"}`), &old); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"id":"b","host":"1.2.3.4","auto_ip_enabled":false}`), &off); err != nil {
		t.Fatal(err)
	}
	if !old.AutoIpEnabled || old.Host != "1.2.3.4" {
		t.Fatalf("a node stored before Auto IP should have it on: %+v", old)
	}
	if off.AutoIpEnabled {
		t.Fatal("a node stored with Auto IP off should keep it off")
	}
}
