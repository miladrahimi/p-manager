package vless

import (
	"slices"
	"testing"
)

func TestMakeRrInboundExcludesItsSniFromSniffing(t *testing.T) {
	in := MakeRrInbound("relay-rr2ssh", 3443, "key", "snapp.ir", nil, nil)
	s := in.Sniffing
	if s == nil || !s.Enabled {
		t.Fatal("sniffing must stay enabled")
	}
	if !slices.Equal(s.DestOverride, []string{"http", "tls", "quic"}) {
		t.Errorf("destOverride = %v", s.DestOverride)
	}
	if s.RouteOnly {
		t.Error("sniffed domains must keep replacing the destination")
	}
	if !slices.Equal(s.DomainsExcluded, []string{"snapp.ir"}) {
		t.Errorf("domainsExcluded = %v, want [snapp.ir]", s.DomainsExcluded)
	}
}
