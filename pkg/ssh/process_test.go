package ssh

import (
	"testing"
	"time"
)

func TestNextRetryDelay(t *testing.T) {
	cases := []struct {
		name      string
		prev, ran time.Duration
		want      time.Duration
	}{
		{"first start", 0, time.Second, minRetryDelay},
		{"quick failure doubles", 2 * time.Second, time.Second, 4 * time.Second},
		{"keeps doubling", 8 * time.Second, 10 * time.Second, 16 * time.Second},
		{"capped", 16 * time.Second, time.Second, maxRetryDelay},
		{"stays capped", maxRetryDelay, time.Second, maxRetryDelay},
		{"died on keepalive still backs off", 4 * time.Second, 45 * time.Second, 8 * time.Second},
		{"stable run resets", maxRetryDelay, stableRun, minRetryDelay},
	}
	for _, c := range cases {
		if got := nextRetryDelay(c.prev, c.ran); got != c.want {
			t.Errorf("%s: nextRetryDelay(%v, %v) = %v, want %v", c.name, c.prev, c.ran, got, c.want)
		}
	}
}
