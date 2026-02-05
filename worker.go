package workermanager

import (
	"context"
	"sync"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

// Handler is the minimal interface for business logic. Handlers are added to
// the work manager and wrapped by a Worker which manages state and lifecycle.
type Handler interface {
	Handle(ctx context.Context)
}

// Worker wraps a Handler and manages its state, status, and lifecycle.
// The manager adds handlers and receives Workers; Worker exposes Start/Stop.
type Worker struct {
	name    string
	handler Handler
	status  Status
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewWorker wraps a handler with the given name into a Worker.
func NewWorker(name string, handler Handler) *Worker {
	return &Worker{
		name:    name,
		handler: handler,
		status:  StatusStopped,
		done:    make(chan struct{}),
	}
}

// Name returns the worker name.
func (w *Worker) Name() string {
	return w.name
}

// Status returns the current worker status.
func (w *Worker) Status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
}

func (w *Worker) setStatus(status Status) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
}

// Start starts the worker loop in a new goroutine. It runs until ctx is
// cancelled. Idempotent if already running (no-op).
func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.status == StatusRunning {
		w.mu.Unlock()
		return nil
	}

	workerCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	w.status = StatusRunning
	w.mu.Unlock()

	go func() {
		defer close(w.done)
		defer w.setStatus(StatusStopped)
		for {
			select {
			case <-workerCtx.Done():
				return
			default:
				w.handler.Handle(workerCtx)
			}
		}
	}()

	return nil
}

// Stop cancels the worker context and returns a channel that closes when
// the worker has stopped.
func (w *Worker) Stop(ctx context.Context) chan struct{} {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel == nil {
		stopped := make(chan struct{})
		close(stopped)
		return stopped
	}

	w.cancel()
	return w.done
}
