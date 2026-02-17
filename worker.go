package smoothoperator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// defaultPanicBackoff is the duration the worker sleeps after recovering a panic
// before calling Handle again.
const defaultPanicBackoff = time.Second

// config holds optional settings for a worker. It is configured via HandlerOption
// when registering a handler with AddHandler.
type config struct {
	maxPanicAttempts  int
	panicBackoff      time.Duration
	messageBufferSize int
	maxDispatchTimeout time.Duration
	messageOnly       bool
}

// HandlerOption configures a worker at registration time. Use WithMaxPanicAttempts,
// WithPanicBackoff, WithMessageBufferSize, WithMaxDispatchTimeout, and WithMessageOnly
// to set optional behavior; zero value applies defaults.
type HandlerOption func(*config)

// WithMaxPanicAttempts sets the maximum number of panic recoveries before the worker
// stops itself. Use 0 for no limit (default).
func WithMaxPanicAttempts(n int) HandlerOption {
	return func(c *config) { c.maxPanicAttempts = n }
}

// WithPanicBackoff sets the duration the worker sleeps after recovering a panic
// before calling Handle again. Use 0 for the default (1 second).
func WithPanicBackoff(d time.Duration) HandlerOption {
	return func(c *config) { c.panicBackoff = d }
}

// WithMessageBufferSize sets the capacity of the worker's incoming message channel.
// Use 0 or 1 for buffer size 1 (default). Larger values allow more messages to queue.
func WithMessageBufferSize(n int) HandlerOption {
	return func(c *config) { c.messageBufferSize = n }
}

// WithMaxDispatchTimeout sets an optional maximum time to wait when sending a message
// to this worker. Use 0 for no timeout (default).
func WithMaxDispatchTimeout(d time.Duration) HandlerOption {
	return func(c *config) { c.maxDispatchTimeout = d }
}

// WithMessageOnly, when true, makes the worker run only when a message is received.
// When false (default), the worker runs in a loop and Handle is called even with no message.
func WithMessageOnly(b bool) HandlerOption {
	return func(c *config) { c.messageOnly = b }
}

// applyHandlerOptions applies the given options to a zero config and returns it.
func applyHandlerOptions(opts ...HandlerOption) config {
	var c config
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// Status represents the current lifecycle state of a Worker.
type Status string

const (
	// StatusRunning means the worker goroutine is active and calling Handle.
	StatusRunning Status = "running"
	// StatusStopped means the worker is not running (either never started or stopped).
	StatusStopped Status = "stopped"
)

// Worker is the metrics view of a worker. Obtain it from Operator.AddHandler or
// Operator.Worker(name). Metrics are only collected after Metrics() or LastMetric()
// is used; the channel is created lazily on first Metrics() call.
type Worker interface {
	// Metrics returns a channel that receives metric events for this worker.
	// The channel is created on first call with the given bufferSize and closed when the worker stops.
	// Receive in a dedicated goroutine to avoid blocking the worker.
	// bufferSize is the capacity of the channel buffer; use 0 for an unbuffered channel.
	Metrics(bufferSize int) <-chan MetricEvent
	// LastMetric returns the most recent metric event and true, or a zero value and false
	// if no event has been recorded yet.
	LastMetric() (MetricEvent, bool)
}

// worker wraps a Handler and manages its state, status, and lifecycle. It runs
// the handler in a loop in a goroutine until stopped. The Operator registers
// handlers and starts/stops workers by name; the worker type is not exposed.
type worker struct {
	name       string
	handler    Handler
	config     config
	status     Status
	mu         sync.Mutex
	cancel     context.CancelFunc
	done       chan struct{}
	panicCount int
	msgCh      chan envelope // buffered channel for incoming messages
	log        *slog.Logger

	metrics metricsRecorder
}

// newWorker creates a worker that runs the given handler under the given name
// with the given config. The worker starts in StatusStopped; call Start to run it.
// logger is the worker's logger (typically a child of the operator's logger with "worker" set).
func newWorker(name string, handler Handler, cfg config, logger *slog.Logger) *worker {
	bufSize := cfg.messageBufferSize
	if bufSize <= 0 {
		bufSize = 1
	}
	return &worker{
		name:    name,
		handler: handler,
		config:  cfg,
		status:  StatusStopped,
		done:    make(chan struct{}),
		msgCh:   make(chan envelope, bufSize),
		log:     logger,
		metrics: newMetricsRecorder(name),
	}
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
		defer w.emitStoppedAndCloseMetrics()
		w.metrics.Record(w.metrics.lifecycleEvent("started"))
		if w.config.messageOnly {
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
func (w *worker) Stop() chan struct{} {
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

	start := time.Now()
	result := w.executeHandler(ctx, w.unwrapEnvelope(raw))
	w.sendResultToEnvelope(raw, result)
	w.metrics.Record(w.metrics.handleEvent(result, time.Since(start), true))
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

	hadMessage := raw.resultCh != nil
	start := time.Now()
	result := w.executeHandler(ctx, w.unwrapEnvelope(raw))
	w.sendResultToEnvelope(raw, result)
	w.metrics.Record(w.metrics.handleEvent(result, time.Since(start), hadMessage))

	if (result.Status == HandleStatusNone || result.Status == HandleStatusFail) && result.IdleDuration > 0 {
		select {
		case <-ctx.Done():
			return
		case raw := <-w.msgCh:
			// Woken up by an incoming message; execute the handler immediately.
			start := time.Now()
			result := w.executeHandler(ctx, w.unwrapEnvelope(raw))
			w.sendResultToEnvelope(raw, result)
			w.metrics.Record(w.metrics.handleEvent(result, time.Since(start), true))
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
		w.log.Error("handle error", "error", result.Err)
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
	w.metrics.Record(w.metrics.panicEvent(w.panicCount, err))
	w.log.Warn("panic recovered", "attempt", w.panicCount, "error", err)

	if w.config.maxPanicAttempts > 0 && w.panicCount >= w.config.maxPanicAttempts {
		w.log.Error("max panic attempts reached, stopping", "attempts", w.config.maxPanicAttempts)
		w.cancel()
		return
	}

	w.mu.Unlock()
	select {
	case <-ctx.Done():
	case <-time.After(w.getPanicBackoff()):
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

// emitStoppedAndCloseMetrics emits a lifecycle "stopped" event then closes the metrics channel.
// Called from the worker goroutine when the run loop exits.
func (w *worker) emitStoppedAndCloseMetrics() {
	w.metrics.Record(w.metrics.lifecycleEvent("stopped"))
	w.metrics.CloseChannel()
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
	return w.config.maxDispatchTimeout
}

// getPanicBackoff returns the effective panic backoff duration (config value or default).
func (w *worker) getPanicBackoff() time.Duration {
	if w.config.panicBackoff > 0 {
		return w.config.panicBackoff
	}
	return defaultPanicBackoff
}
