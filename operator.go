package smoothoperator

import (
	"context"
	"fmt"
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
}

// Operator manages a set of named workers. Register handlers with AddHandler,
// then start/stop workers by name or all at once. All methods are safe for
// concurrent use.
type Operator interface {
	// AddHandler registers a handler under the given name with the given config.
	// Returns an error if name is already registered.
	AddHandler(name string, handler Handler, config Config) error
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
}

type operator struct {
	ctx     context.Context
	workers map[string]*worker
	mu      sync.RWMutex
}

// NewOperator creates an Operator that will use ctx for worker lifecycle. Workers
// started via this operator run until ctx is cancelled or Stop/StopAll is called.
func NewOperator(ctx context.Context) Operator {
	return &operator{
		ctx:     ctx,
		workers: make(map[string]*worker),
		mu:      sync.RWMutex{},
	}
}

func (op *operator) AddHandler(name string, handler Handler, config Config) error {
	op.mu.Lock()
	defer op.mu.Unlock()

	if _, ok := op.workers[name]; ok {
		return fmt.Errorf("worker %s already exists", name)
	}

	op.workers[name] = newWorker(name, handler, config)
	return nil
}

func (op *operator) RemoveHandler(name string) error {
	op.mu.Lock()
	w, ok := op.workers[name]
	if !ok {
		op.mu.Unlock()
		return fmt.Errorf("worker %s not found", name)
	}
	delete(op.workers, name)
	op.mu.Unlock()

	// Stop the worker (if running) and wait for it to finish.
	<-w.Stop(context.Background())
	return nil
}

func (op *operator) Status(name string) (Status, error) {
	op.mu.RLock()
	defer op.mu.RUnlock()

	w, ok := op.workers[name]
	if !ok {
		return "", fmt.Errorf("worker %s not found", name)
	}
	return w.getStatus(), nil
}

func (op *operator) Start(name string) error {
	op.mu.RLock()
	defer op.mu.RUnlock()

	worker, ok := op.workers[name]
	if !ok {
		return fmt.Errorf("worker %s not found", name)
	}

	return worker.Start(op.ctx)
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
			return err
		}
	}
	return nil
}

func (op *operator) Stop(name string) (chan struct{}, error) {
	op.mu.RLock()
	defer op.mu.RUnlock()

	worker, ok := op.workers[name]
	if !ok {
		return nil, fmt.Errorf("worker %s not found", name)
	}

	return worker.Stop(context.Background()), nil
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
