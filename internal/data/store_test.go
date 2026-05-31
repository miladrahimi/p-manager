package data

import (
	"sync"
	"testing"

	"github.com/miladrahimi/p-node/pkg/database"
)

// TestStoreConcurrentAccess exercises the Store under concurrent readers,
// writers, and backups. Run with -race to catch unsynchronized access to the
// shared data graph.
func TestStoreConcurrentAccess(t *testing.T) {
	db, err := database.New[Data](t.TempDir(), Default())
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	store := NewStore(db)

	const workers = 16
	const iterations = 200

	var wg sync.WaitGroup

	// Writers: append accounts and bump shared stats.
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = store.Write(func(d *Data) {
					d.Accounts = append(d.Accounts, &Account{Id: "x"})
					d.Stats.TotalUsageBytes += 1
				})
			}
		}()
	}

	// Readers: iterate the live slice and the stats.
	for r := 0; r < workers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				store.Read(func(d *Data) {
					total := int64(0)
					for range d.Accounts {
						total++
					}
					_ = total + d.Stats.TotalUsageBytes
				})
			}
		}()
	}

	// Backups: marshal the graph concurrently with the writers.
	for b := 0; b < 2; b++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = store.Backup()
			}
		}()
	}

	wg.Wait()
}
