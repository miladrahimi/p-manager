package coordinator

import (
	"strconv"
	"sync"
	"testing"

	"github.com/miladrahimi/p-manager/pkg/ssh"
)

// TestStateConcurrentAccess hammers the coordinator State from many goroutines.
// Before the mutex was added this reproduced "fatal error: concurrent map read
// and map write". Run with -race.
func TestStateConcurrentAccess(t *testing.T) {
	s := newState()

	const workers = 16
	const iterations = 500

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			id := strconv.Itoa(w)
			for i := 0; i < iterations; i++ {
				s.SetSshConfigs(id, []*ssh.ProxyConfig{{LocalPort: i + 1}})
				s.SshConfigs(id)
				for range s.SshConfigsByNode() {
				}
				_ = s.SshConfigsCount()
				s.SetXrayUpdatedAt(s.XrayUpdatedAt())
				s.RemoveSshConfigs(id)
			}
		}(w)
	}
	wg.Wait()
}
