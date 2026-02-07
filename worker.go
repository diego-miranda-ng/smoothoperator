package smoothoperator

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// defaultPanicBackoff is the duration the worker sleeps after recovering a panic
// before calling Handle again.
const defaultPanicBackoff = time.Second

// Status represents the current lifecycle state of a Worker.
type Status string

const (
	// StatusRunning means the worker goroutine is active and calling Handle.
	StatusRunning Status = "running"
	// StatusStopped means the worker is not running (either never started or stopped).
	StatusStopped Status = "stopped"
)

// worker wraps a Handler and manages its state, status, and lifecycle. It runs
// the handler in a loop in a goroutine until stopped. The Operator registers
// handlers and starts/stops workers by name; the worker type is not exposed.
type worker struct {
	name             string
	handler          Handler
	status           Status
	mu               sync.Mutex
	cancel           context.CancelFunc
	done             chan struct{}
	panicCount       int
	maxPanicAttempts int           // 0 means no limit
	panicBackoff     time.Duration // 0 means use defaultPanicBackoff
}

// newWorker creates a worker that runs the given handler under the given name
// with the given config. The worker starts in StatusStopped; call Start to run it.
func newWorker(name string, handler Handler, config Config) *worker {
	backoff := config.PanicBackoff
	if backoff <= 0 {
		backoff = defaultPanicBackoff
	}
	return &worker{
		name:             name,
		handler:          handler,
		status:           StatusStopped,
		done:             make(chan struct{}),
		maxPanicAttempts: config.MaxPanicAttempts,
		panicBackoff:     backoff,
	}
}

func (w *worker) getName() string {
	return w.name
}

// getStatus returns the current worker status. Safe to call from any goroutine.
func (w *worker) getStatus() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.status
}

func (w *worker) setStatus(s Status) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = s
}

// Start starts the worker loop in a new goroutine. The loop runs until ctx is
// cancelled (e.g. by calling Stop). Idempotent: if the worker is already
// running, Start returns nil without starting a second loop.
func (w *worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.status == StatusRunning {
		w.mu.Unlock()
		return nil
	}

	workerCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.done = make(chan struct{})
	w.status = StatusRunning
	w.panicCount = 0
	w.mu.Unlock()

	go func() {
		defer close(w.done)
		defer w.setStatus(StatusStopped)
		for {
			select {
			case <-workerCtx.Done():
				return
			default:
				w.handle(workerCtx)
			}
		}
	}()

	return nil
}

// Stop cancels the worker's context and returns a channel that closes when
// the worker goroutine has fully exited. If the worker was never started,
// returns an already-closed channel immediately.
func (w *worker) Stop(ctx context.Context) chan struct{} {
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

func (w *worker) handle(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	defer func() {
		if v := recover(); v != nil {
			w.onPanicRecovered(ctx, v)
		}
	}()

	result := w.handler.Handle(ctx)
	if result.Status == HandleStatusFail && result.Err != nil {
		log.Printf("worker %s: handle error: %v", w.name, result.Err)
	}

	if (result.Status == HandleStatusNone || result.Status == HandleStatusFail) && result.IdleDuration > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(result.IdleDuration):
		}
	}
}

// onPanicRecovered is run from a defer in handle() after recovering a panic.
// Caller holds w.mu. It logs, increments count, stops if max reached, and does backoff (releasing lock during sleep).
func (w *worker) onPanicRecovered(ctx context.Context, v interface{}) {
	err := panicToError(v)
	w.panicCount++
	log.Printf("worker %s: panic recovered (attempt %d): %v", w.name, w.panicCount, err)

	if w.maxPanicAttempts > 0 && w.panicCount >= w.maxPanicAttempts {
		log.Printf("worker %s: max panic attempts (%d) reached, stopping", w.name, w.maxPanicAttempts)
		w.cancel()
		return
	}

	w.mu.Unlock()
	select {
	case <-ctx.Done():
	case <-time.After(w.panicBackoff):
	}
	w.mu.Lock()
}

// panicToError converts a recovered panic value to an error for logging.
func panicToError(v interface{}) error {
	if err, ok := v.(error); ok {
		return err
	}
	return fmt.Errorf("panic: %v", v)
}
