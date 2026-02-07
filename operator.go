package smoothoperator

import (
	"context"
	"fmt"
	"sync"
)

// Operator manages a set of named workers. Register handlers with AddHandler,
// then start/stop workers by name or all at once. All methods are safe for
// concurrent use.
type Operator interface {
	// AddHandler registers a handler under the given name and returns the Worker.
	// Returns an error if name is already registered.
	AddHandler(name string, handler Handler) (*Worker, error)
	// Start starts the worker with the given name. Returns error if name not found.
	Start(name string) error
	// StartAll starts every registered worker. Returns on first error if any.
	StartAll() error
	// Stop stops the worker with the given name and returns a channel that closes
	// when the worker has stopped. Returns (nil, error) if name not found.
	Stop(name string) (chan struct{}, error)
	// StopAll stops all workers and returns a channel that closes when all have stopped.
	StopAll() chan struct{}
}

type operator struct {
	ctx     context.Context
	workers map[string]*Worker
	mu      sync.RWMutex
}

// NewOperator creates an Operator that will use ctx for worker lifecycle. Workers
// started via this operator run until ctx is cancelled or Stop/StopAll is called.
func NewOperator(ctx context.Context) Operator {
	return &operator{
		ctx:     ctx,
		workers: make(map[string]*Worker),
		mu:      sync.RWMutex{},
	}
}

func (op *operator) AddHandler(name string, handler Handler) (*Worker, error) {
	op.mu.Lock()
	defer op.mu.Unlock()

	if _, ok := op.workers[name]; ok {
		return nil, fmt.Errorf("worker %s already exists", name)
	}

	worker := NewWorker(name, handler)
	op.workers[name] = worker

	return worker, nil
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
	defer op.mu.RUnlock()

	for _, worker := range op.workers {
		if err := op.Start(worker.Name()); err != nil {
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
	defer op.mu.RUnlock()

	for _, worker := range op.workers {
		if stopChan, _ := op.Stop(worker.Name()); stopChan != nil {
			<-stopChan
		}
	}

	stopped := make(chan struct{})
	close(stopped)
	return stopped
}
