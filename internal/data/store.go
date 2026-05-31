package data

import (
	"sync"

	"github.com/miladrahimi/p-node/pkg/database"
)

// Store wraps the JSON database with a read/write lock so the in-memory data
// graph can be shared safely between the coordinator workers (and their
// fan-out goroutines) and the HTTP handlers.
//
// The embedded *database.Database only serializes file I/O; it hands out the
// shared *Data pointer without guarding concurrent access to it. Store closes
// that gap: every read of and mutation to the data graph must go through the
// helpers below.
//
// Locking discipline: critical sections must stay short and CPU-only. Never
// hold the lock across network calls, xray reconfiguration, ssh process
// management, or coordinator.UpdateConfigs, and never call a method that
// re-acquires the same lock (the composer locks internally, so it must not be
// invoked from inside Read/Write/Mutate).
type Store struct {
	*database.Database[Data]
	mu sync.RWMutex
}

// NewStore wraps the given database.
func NewStore(db *database.Database[Data]) *Store {
	return &Store{Database: db}
}

// Read runs fn while holding the read lock.
func (s *Store) Read(fn func(d *Data)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.Database.Data())
}

// Write runs fn while holding the write lock and then persists the data.
func (s *Store) Write(fn func(d *Data)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.Database.Data())
	return s.Database.Save()
}

// Mutate runs fn while holding the write lock and persists only when fn returns
// save=true. Use it for handlers that may early-return without changing data
// (e.g. validation failures, "not found").
func (s *Store) Mutate(fn func(d *Data) (save bool, err error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	save, err := fn(s.Database.Data())
	if err != nil {
		return err
	}
	if !save {
		return nil
	}
	return s.Database.Save()
}

// Save persists the data while holding the write lock. Prefer Write/Mutate; use
// this only for save-only paths (e.g. graceful shutdown).
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Database.Save()
}

// Backup writes a backup file while holding the read lock so the marshal does
// not race with a concurrent mutation.
func (s *Store) Backup() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Database.Backup()
}
