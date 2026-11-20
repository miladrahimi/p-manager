package coordinator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/miladrahimi/p-manager/internal/data"
	"github.com/miladrahimi/p-node/pkg/logger"
	"github.com/miladrahimi/p-node/pkg/xray"
)

// A fresh manager has nothing to serve, so Xray is not started yet; reading
// its stats then must be a quiet no-op, not a dial error every minute.
func TestLoadLocalStatsSkipsStoppedXray(t *testing.T) {
	// The logger writes under ./storage/logs; run from a scratch directory.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "storage", "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	l, err := logger.New("debug", "2006-01-02 15:04:05.000", make(chan struct{}))
	if err != nil {
		t.Fatal(err)
	}
	x := xray.New(context.Background(), l, "warning", filepath.Join(dir, "xray.json"), filepath.Join(dir, "xray"))
	s := newStatsSyncer(l, nil, nil, x, nil)
	if err := s.loadLocalStats(); err != nil {
		t.Fatalf("loadLocalStats with Xray not running = %v, want nil", err)
	}
}

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
