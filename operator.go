package smoothoperator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Config holds optional settings for a worker registered with AddHandler.
type Config struct {
	// MaxPanicAttempts is the maximum number of panic recoveries before the worker
	// stops itself. Use 0 for no limit (default).
	MaxPanicAttempts int
	// PanicBackoff is the duration the worker sleeps after recovering a panic
	// before calling Handle again. Use 0 for the default (1 second).
	PanicBackoff time.Duration
	// MessageBufferSize is the capacity of the worker's incoming message channel.
	// When the buffer is full, Dispatch blocks until the worker reads a message or
	// ctx is done. Use 0 or 1 for buffer size 1 (default). Larger values allow
	// more messages to queue; ordering is FIFO and backpressure is applied when full.
	MessageBufferSize int
	// MaxDispatchTimeout is an optional maximum time to wait when sending a message
	// to this worker. If the send would block longer (e.g. buffer full), Dispatch
	// returns context.DeadlineExceeded. Use 0 for no timeout (default); the send
	// is then limited only by the context passed to Dispatch.
	MaxDispatchTimeout time.Duration
	// MessageOnly, when true, makes the worker run only when a message is received.
	// Handle is never called with nil; the worker blocks on the message channel
	// until a message is dispatched. When false (default), the worker runs in a
	// loop and Handle is called even with no message (msg == nil), as today.
	MessageOnly bool
}

type Dispatcher interface {
	// Dispatch sends a message to the worker with the given name. If the worker is
	// idle, it wakes up immediately. The message is passed to Handle via the msg
	// parameter. If the worker's message buffer is full, Dispatch blocks until
	// there is space or ctx is done (e.g. timeout or cancel); on ctx.Done it
	// returns ctx.Err() and does not send. When Config.MaxDispatchTimeout is set
	// for the worker, the send is also bounded by that duration. Returns: a
	// channel that closes once the handler has received the message; a channel
	// that receives the handler's Result (HandleResult.Result) when the handler
	// finishes, then closes; and an error if the worker is not found or ctx was
	// cancelled (including MaxDispatchTimeout). Prefer SendMessage or
	// SendMessageWithContext for type-safe sending.
	Dispatch(ctx context.Context, name string, msg any) (delivered <-chan struct{}, result <-chan any, err error)
}

// Operator manages a set of named workers. Register handlers with AddHandler,
// then start/stop workers by name or all at once. All methods are safe for
// concurrent use.
type Operator interface {
	Dispatcher
	// AddHandler registers a handler under the given name with the given config.
	// Returns the Worker interface for metrics and an error if name is already registered.
	AddHandler(name string, handler Handler, config Config) (Worker, error)
	// Start starts the worker with the given name. Returns error if name not found.
	Start(name string) error
	// StartAll starts every registered worker. Returns on first error if any.
	StartAll() error
	// Stop stops the worker with the given name and returns a channel that closes
	// when the worker has stopped. Returns (nil, error) if name not found.
	Stop(name string) (chan struct{}, error)
	// StopAll stops all workers and returns a channel that closes when all have stopped.
	StopAll() chan struct{}
	// RemoveHandler stops the worker with the given name (if running), waits for it
	// to finish, and removes it from the operator. Returns error if name not found.
	RemoveHandler(name string) error
	// Status returns the current status of the worker with the given name.
	// Returns error if name not found.
	Status(name string) (Status, error)
	// Worker returns the Worker interface for the given name, for metrics (Metrics channel and LastMetric).
	// Returns error if name not found.
	Worker(name string) (Worker, error)
}

// defaultLogger is the logger used when no WithLogger option is provided. It
// writes JSON to os.Stdout.
var defaultLogger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

// Option configures an Operator at creation time. Use WithLogger to supply a
// custom logger; otherwise a default JSON logger writing to os.Stdout is used.
// Logs from the operator and its workers form a tree: operator logs use the
// given logger, and each worker uses a child logger with a "worker" attribute
// set to the worker name.
type Option func(*operator)

// WithLogger sets the logger used by the operator and all its workers. Each
// worker gets a child logger with "worker" set to the worker name. If logger is
// nil, the default JSON logger (writing to os.Stdout) is used.
func WithLogger(logger *slog.Logger) Option {
	return func(o *operator) {
		if logger != nil {
			o.log = logger
		}
	}
}

type operator struct {
	ctx     context.Context
	log     *slog.Logger
	workers map[string]*worker
	mu      sync.RWMutex
}

// NewOperator creates an Operator that will use ctx for worker lifecycle. Workers
// started via this operator run until ctx is cancelled or Stop/StopAll is called.
// Optional options (e.g. WithLogger) configure the operator; if no logger is
// provided, a default JSON logger writing to os.Stdout is used.
func NewOperator(ctx context.Context, opts ...Option) Operator {
	o := &operator{
		ctx:     ctx,
		log:     defaultLogger,
		workers: make(map[string]*worker),
		mu:      sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (op *operator) AddHandler(name string, handler Handler, config Config) (Worker, error) {
	op.mu.Lock()
	defer op.mu.Unlock()

	if _, ok := op.workers[name]; ok {
		return nil, op.errorHandler(fmt.Errorf("worker %s already exists", name))
	}

	w := newWorker(name, handler, config, op.log.With("worker", name))
	op.workers[name] = w
	if aware, ok := handler.(DispatcherAware); ok {
		aware.SetDispatcher(op)
	}
	op.log.Info("handler added", "worker", name)
	return &w.metrics, nil
}

func (op *operator) RemoveHandler(name string) error {
	op.mu.Lock()
	w, ok := op.workers[name]
	if !ok {
		op.mu.Unlock()
		return op.errorHandler(fmt.Errorf("worker %s not found", name))
	}
	delete(op.workers, name)
	op.mu.Unlock()

	op.log.Info("handler removed", "worker", name)
	// Stop the worker (if running) and wait for it to finish.
	<-w.Stop()
	return nil
}

func (op *operator) Dispatch(ctx context.Context, name string, msg any) (<-chan struct{}, <-chan any, error) {
	op.mu.RLock()
	w, ok := op.workers[name]
	op.mu.RUnlock()

	if !ok {
		return nil, nil, op.errorHandler(fmt.Errorf("worker %s not found", name))
	}

	sendCtx := ctx
	if max := w.getMaxDispatchTimeout(); max > 0 {
		var cancel context.CancelFunc
		sendCtx, cancel = context.WithTimeout(ctx, max)
		defer cancel()
	}

	env := envelope{
		msg:       msg,
		delivered: make(chan struct{}),
		resultCh:  make(chan any, 1),
	}
	select {
	case w.msgCh <- env:
		w.metrics.Record(w.metrics.dispatchEvent(true, nil))
		return env.delivered, env.resultCh, nil
	case <-sendCtx.Done():
		w.metrics.Record(w.metrics.dispatchEvent(false, sendCtx.Err()))
		return nil, nil, op.errorHandler(fmt.Errorf("dispatch timeout: %w", sendCtx.Err()))
	}
}

func (op *operator) Status(name string) (Status, error) {
	op.mu.RLock()
	defer op.mu.RUnlock()

	w, ok := op.workers[name]
	if !ok {
		return "", op.errorHandler(fmt.Errorf("worker %s not found", name))
	}
	return w.getStatus(), nil
}

func (op *operator) Worker(name string) (Worker, error) {
	op.mu.RLock()
	defer op.mu.RUnlock()

	w, ok := op.workers[name]
	if !ok {
		return nil, op.errorHandler(fmt.Errorf("worker %s not found", name))
	}
	return &w.metrics, nil
}

func (op *operator) Start(name string) error {
	op.mu.RLock()
	defer op.mu.RUnlock()

	worker, ok := op.workers[name]
	if !ok {
		return op.errorHandler(fmt.Errorf("worker %s not found", name))
	}

	err := worker.Start(op.ctx)
	if err == nil {
		op.log.Debug("worker started", "worker", name)
	}
	return op.errorHandler(err)
}

func (op *operator) StartAll() error {
	op.mu.RLock()
	names := make([]string, 0, len(op.workers))
	for name := range op.workers {
		names = append(names, name)
	}
	op.mu.RUnlock()

	for _, name := range names {
		if err := op.Start(name); err != nil {
			return op.errorHandler(err)
		}
	}
	return nil
}

func (op *operator) Stop(name string) (chan struct{}, error) {
	op.mu.RLock()
	defer op.mu.RUnlock()

	worker, ok := op.workers[name]
	if !ok {
		return nil, op.errorHandler(fmt.Errorf("worker %s not found", name))
	}

	ch := worker.Stop()
	op.log.Debug("worker stop requested", "worker", name)
	return ch, nil
}

func (op *operator) StopAll() chan struct{} {
	op.mu.RLock()
	names := make([]string, 0, len(op.workers))
	for name := range op.workers {
		names = append(names, name)
	}
	op.mu.RUnlock()

	for _, name := range names {
		if stopChan, _ := op.Stop(name); stopChan != nil {
			<-stopChan
		}
	}

	stopped := make(chan struct{})
	close(stopped)
	return stopped
}

// errorHandler logs the error with the operator's logger and returns the same error.
// If err is nil, it returns nil without logging.
func (op *operator) errorHandler(err error) error {
	if err != nil {
		op.log.Error("operator error", "error", err)
	}
	return err
}
