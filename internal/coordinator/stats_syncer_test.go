package coordinator

import (
	"testing"

	"github.com/miladrahimi/p-manager/internal/data"
)

func TestNodeStatsEnabled(t *testing.T) {
	cases := []struct {
		name string
		xs   data.XraySettings
		want bool
	}{
		{"nothing on the nodes", data.XraySettings{RelayRr2RrNodePort: 443, ReverseRrPortalPort: 8443}, false},
		{"remote rr", data.XraySettings{RemoteRrPort: 443}, true},
		{"remote cxh only", data.XraySettings{RemoteCxhPort: 2053, RelayRr2RrNodePort: 443}, true},
		{"both", data.XraySettings{RemoteRrPort: 443, RemoteCxhPort: 2053}, true},
	}
	for _, c := range cases {
		if got := nodeStatsEnabled(&c.xs); got != c.want {
			t.Errorf("%s: nodeStatsEnabled = %v, want %v", c.name, got, c.want)
		}
	}
}
