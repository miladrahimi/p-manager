package coordinator

import (
	"context"
	"time"
)

type Worker struct {
	context  context.Context
	interval time.Duration
	body     func()
	callback func()
}

func (w *Worker) Start() {
	ticker := time.NewTicker(w.interval)
	go func() {
		for {
			select {
			case <-w.context.Done():
				w.callback()
				return
			case <-ticker.C:
				w.body()
			}
		}
	}()
}

func newWorker(c context.Context, interval time.Duration, body func(), callback func()) *Worker {
	return &Worker{context: c, interval: interval, body: body, callback: callback}
}
