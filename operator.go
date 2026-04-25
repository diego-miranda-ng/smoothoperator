package smoothoperator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// Dispatcher sends messages to named workers. The Operator implements this
// interface; handlers that implement DispatcherAware receive it at registration
// time so they can forward messages to other workers.
type Dispatcher interface {
	// Dispatch sends a message to the worker with the given name. If the worker is
	// idle, it wakes up immediately. The message is passed to Handle via the msg
	// parameter. If the worker's message buffer is full, Dispatch blocks until
	// there is space or ctx is done (e.g. timeout or cancel); on ctx.Done it
	// returns ctx.Err() and does not send. When WithMaxDispatchTimeout is set
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
	// AddHandler registers a handler under the given name. Optional HandlerOption
	// values (e.g. WithMaxPanicAttempts, WithMessageOnly) configure the worker.
	// Returns the Worker interface for metrics and an error if name is already registered.
	AddHandler(name string, handler Handler, opts ...HandlerOption) (Worker, error)
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
	// Worker returns the Worker interface for the given name, for per-kind metrics channels.
	// Returns error if name not found.
	Worker(name string) (Worker, error)
	// UpdateHandlerOptions applies the given options on top of the worker's current
	// configuration. If the worker is running it is stopped first, the config is
	// updated, and the worker is restarted. If the worker is stopped it stays
	// stopped after the update. Returns ErrWorkerNotFound if name is not registered.
	UpdateHandlerOptions(name string, opts ...HandlerOption) error
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

// AddHandler registers a handler under name with the given options. The worker
// is created in StatusStopped; call Start or StartAll to run it. If the handler
// implements DispatcherAware, SetDispatcher is called with the operator before
// returning. Returns ErrWorkerAlreadyExists if name is already registered and
// ErrNilHandler if handler is nil.
func (op *operator) AddHandler(name string, handler Handler, opts ...HandlerOption) (Worker, error) {
	op.mu.Lock()
	defer op.mu.Unlock()

	if _, ok := op.workers[name]; ok {
		return nil, op.errorHandler(fmt.Errorf("worker %s already exists: %w", name, ErrWorkerAlreadyExists))
	}
	if handler == nil {
		return nil, op.errorHandler(fmt.Errorf("worker %s has nil handler: %w", name, ErrNilHandler))
	}

	cfg := applyHandlerOptions(config{}, opts...)
	w := newWorker(name, handler, cfg, op.log)
	op.workers[name] = w
	if aware, ok := handler.(DispatcherAware); ok {
		aware.SetDispatcher(op)
	}
	op.log.Info("handler added", "worker", name)

	return w, nil
}

// RemoveHandler unregisters the named worker, stopping it first if running and
// waiting for the goroutine to exit. After removal, the name can be reused with
// AddHandler. Returns ErrWorkerNotFound if name is not registered.
func (op *operator) RemoveHandler(name string) error {
	op.mu.Lock()
	w, err := op.getWorkerLocked(name)
	if err != nil {
		op.mu.Unlock()
		return err
	}
	delete(op.workers, name)
	op.mu.Unlock()

	op.log.Info("handler removed", "worker", name)
	// Stop the worker (if running) and wait for it to finish.
	<-w.stop()
	return nil
}

// UpdateHandlerOptions applies opts on top of the named worker's current config.
// If the worker is running it is stopped, reconfigured, and restarted. If the
// worker is stopped it remains stopped after the config change. Returns
// ErrWorkerNotFound if name is not registered.
func (op *operator) UpdateHandlerOptions(name string, opts ...HandlerOption) error {
	w, err := op.getWorker(name)
	if err != nil {
		return err
	}

	wasRunning := w.getStatus() == StatusRunning
	if wasRunning {
		<-w.stop()
	}

	cfg := applyHandlerOptions(w.config, opts...)
	w.applyConfig(cfg, op.log)
	op.log.Info("handler options updated", "worker", name)

	if wasRunning {
		if err := w.start(op.ctx); err != nil {
			return op.errorHandler(err)
		}
	}
	return nil
}

// Dispatch sends msg to the named worker's message channel. If the worker has a
// WithMaxDispatchTimeout configured, a derived context with that timeout bounds
// the send. Returns a delivered channel (closed when the handler dequeues the
// message), a result channel (receives HandleResult.Result then closes), and an
// error (ErrWorkerNotFound or ErrDispatchTimeout joined with the context error).
func (op *operator) Dispatch(ctx context.Context, name string, msg any) (<-chan struct{}, <-chan any, error) {
	w, err := op.getWorker(name)
	if err != nil {
		return nil, nil, err
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
	if w.sendEnvelope(sendCtx, env) {
		return env.delivered, env.resultCh, nil
	}
	return nil, nil, op.errorHandler(errors.Join(ErrDispatchTimeout, fmt.Errorf("dispatch timeout: %w", sendCtx.Err())))
}

// Status returns the current lifecycle state of the named worker (StatusRunning
// or StatusStopped). Returns ErrWorkerNotFound if name is not registered.
func (op *operator) Status(name string) (Status, error) {
	w, err := op.getWorker(name)
	if err != nil {
		return "", err
	}
	return w.getStatus(), nil
}

// Worker returns the Worker interface for the named worker, providing access to
// Name, Status, and streaming Metrics channels. Returns ErrWorkerNotFound if
// name is not registered.
func (op *operator) Worker(name string) (Worker, error) {
	w, err := op.getWorker(name)
	if err != nil {
		return nil, err
	}

	return w, nil
}

// Start launches the named worker's goroutine. Returns ErrWorkerNotFound if
// name is not registered and ErrWorkerAlreadyRunning if the worker is already
// running.
func (op *operator) Start(name string) error {
	w, err := op.getWorker(name)
	if err != nil {
		return err
	}

	if err := w.start(op.ctx); err != nil {
		return op.errorHandler(err)
	}
	op.log.Debug("worker started", "worker", name)
	return nil
}

// StartAll iterates over every registered worker and calls Start. If any Start
// returns an error, it is returned immediately and remaining workers are not
// started.
func (op *operator) StartAll() error {
	for _, name := range op.getWorkerNames() {
		if err := op.Start(name); err != nil {
			return op.errorHandler(err)
		}
	}
	return nil
}

// Stop cancels the named worker's context and returns a channel that closes when
// the goroutine has fully exited. Returns ErrWorkerNotFound if name is not
// registered. Block on the returned channel to wait for a clean shutdown.
func (op *operator) Stop(name string) (chan struct{}, error) {
	w, err := op.getWorker(name)
	if err != nil {
		return nil, err
	}

	ch := w.stop()
	op.log.Debug("worker stop requested", "worker", name)
	return ch, nil
}

// StopAll stops every registered worker and waits for each goroutine to exit
// sequentially. Returns a closed channel once all workers have stopped.
func (op *operator) StopAll() chan struct{} {
	for _, name := range op.getWorkerNames() {
		if stopChan, _ := op.Stop(name); stopChan != nil {
			<-stopChan
		}
	}

	stopped := make(chan struct{})
	close(stopped)
	return stopped
}

// getWorkerNames returns a slice containing the names of all registered workers.
// Safe for concurrent use.
func (op *operator) getWorkerNames() []string {
	op.mu.RLock()
	defer op.mu.RUnlock()

	names := make([]string, 0, len(op.workers))
	for name := range op.workers {
		names = append(names, name)
	}
	return names
}

// getWorker returns the worker with the given name, or nil and an error if not found.
// Caller must not hold op.mu. Safe for concurrent use.
func (op *operator) getWorker(name string) (*worker, error) {
	op.mu.RLock()
	w, err := op.getWorkerLocked(name)
	op.mu.RUnlock()

	return w, err
}

// getWorkerLocked returns the worker with the given name, or nil and an error if not found.
// Caller must hold op.mu (read or write). Use when the caller already holds the lock (e.g. RemoveHandler).
func (op *operator) getWorkerLocked(name string) (*worker, error) {
	w, ok := op.workers[name]
	if !ok {
		return nil, op.errorHandler(fmt.Errorf("worker %s not found: %w", name, ErrWorkerNotFound))
	}
	return w, nil
}

// errorHandler logs the error with the operator's logger and returns the same error.
// If err is nil, it returns nil without logging.
func (op *operator) errorHandler(err error) error {
	if err != nil {
		op.log.Error("operator error", "error", err)
	}
	return err
}
