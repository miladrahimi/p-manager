package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/miladrahimi/p-node/pkg/logger"
)

// Worker is a struct that represents a periodic worker.
type Worker struct {
	name     string
	interval time.Duration
	l        *logger.Logger
	body     func()
}

// New creates a new worker.
func New(name string, interval time.Duration, l *logger.Logger, body func()) *Worker {
	return &Worker{name: name, interval: interval, l: l, body: body}
}

// Start starts the worker.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				if w.l != nil {
					w.l.Info(fmt.Sprintf("worker '%s': stopped", w.name))
				}
				return
			case <-ticker.C:
				if w.l != nil {
					w.l.Info(fmt.Sprintf("worker '%s': running...", w.name))
				}
				w.body()
			}
		}
	}()
}
