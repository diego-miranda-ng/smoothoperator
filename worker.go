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
	name               string
	handler            Handler
	status             Status
	mu                 sync.Mutex
	cancel             context.CancelFunc
	done               chan struct{}
	panicCount         int
	maxPanicAttempts   int           // 0 means no limit
	panicBackoff       time.Duration // 0 means use defaultPanicBackoff
	maxDispatchTimeout time.Duration // 0 means no timeout
	messageOnly        bool          // if true, only run when a message is received
	msgCh              chan envelope // buffered channel for incoming messages
}

// newWorker creates a worker that runs the given handler under the given name
// with the given config. The worker starts in StatusStopped; call Start to run it.
func newWorker(name string, handler Handler, config Config) *worker {
	backoff := config.PanicBackoff
	if backoff <= 0 {
		backoff = defaultPanicBackoff
	}
	bufSize := config.MessageBufferSize
	if bufSize <= 0 {
		bufSize = 1
	}
	return &worker{
		name:               name,
		handler:            handler,
		status:             StatusStopped,
		done:               make(chan struct{}),
		maxPanicAttempts:   config.MaxPanicAttempts,
		panicBackoff:       backoff,
		maxDispatchTimeout: config.MaxDispatchTimeout,
		messageOnly:        config.MessageOnly,
		msgCh:              make(chan envelope, bufSize),
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

// getMaxDispatchTimeout returns the worker's max dispatch timeout (0 means no limit).
// Safe to call from any goroutine; the value is set at construction and never changed.
func (w *worker) getMaxDispatchTimeout() time.Duration {
	return w.maxDispatchTimeout
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
		if w.messageOnly {
			w.runMessageOnly(workerCtx)
		} else {
			w.runLoop(workerCtx)
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

// runLoop runs the worker in polling mode: Handle is called repeatedly, with or without a message.
func (w *worker) runLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			w.handle(ctx)
		}
	}
}

// runMessageOnly runs the worker in message-only mode: Handle is called only when a message is received.
func (w *worker) runMessageOnly(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case raw := <-w.msgCh:
			w.processEnvelope(ctx, raw)
		}
	}
}

// processEnvelope runs the handler for one envelope (with panic recovery) and sends the result.
// Caller must not hold w.mu. Returns the HandleResult for use by handle() when checking IdleDuration.
func (w *worker) processEnvelope(ctx context.Context, raw envelope) HandleResult {
	w.mu.Lock()
	defer w.mu.Unlock()
	defer w.onPanicRecovered(ctx)

	result := w.executeHandler(ctx, w.unwrapEnvelope(raw))
	w.sendResultToEnvelope(raw, result)
	return result
}

func (w *worker) handle(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	defer w.onPanicRecovered(ctx)

	// Non-blocking check for a pending message.
	var raw envelope
	select {
	case raw = <-w.msgCh:
	default:
	}

	result := w.executeHandler(ctx, w.unwrapEnvelope(raw))
	w.sendResultToEnvelope(raw, result)

	if (result.Status == HandleStatusNone || result.Status == HandleStatusFail) && result.IdleDuration > 0 {
		select {
		case <-ctx.Done():
			return
		case raw := <-w.msgCh:
			// Woken up by an incoming message; execute the handler immediately.
			result := w.executeHandler(ctx, w.unwrapEnvelope(raw))
			w.sendResultToEnvelope(raw, result)
		case <-time.After(result.IdleDuration):
		}
	}
}

// unwrapEnvelope extracts the message from an envelope and signals delivery by
// closing the delivered channel. Returns nil when env is a zero-value (no
// message was received).
func (w *worker) unwrapEnvelope(env envelope) any {
	if env.delivered != nil {
		close(env.delivered)
	}
	return env.msg
}

// sendResultToEnvelope sends the handler's Result to the envelope's result channel,
// then closes it. No-op when env.resultCh is nil (no message was received).
func (w *worker) sendResultToEnvelope(env envelope, result HandleResult) {
	if env.resultCh != nil {
		env.resultCh <- result.Result
		close(env.resultCh)
	}
}

// executeHandler calls the handler's Handle method and logs any failure error.
func (w *worker) executeHandler(ctx context.Context, msg any) HandleResult {
	result := w.handler.Handle(ctx, msg)
	if result.Status == HandleStatusFail && result.Err != nil {
		log.Printf("worker %s: handle error: %v", w.name, result.Err)
	}
	return result
}

// onPanicRecovered is run from a defer in handle() after recovering a panic.
// Caller holds w.mu. It logs, increments count, stops if max reached, and does backoff (releasing lock during sleep).
func (w *worker) onPanicRecovered(ctx context.Context) {
	var v interface{}
	if v = recover(); v == nil {
		return
	}

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
