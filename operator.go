package workermanager

import (
	"context"
	"fmt"
	"sync"
)

type Operator interface {
	AddHandler(name string, handler Handler) (*Worker, error)
	Start(name string) error
	StartAll() error
	Stop(name string) (chan struct{}, error)
	StopAll() chan struct{}
}

type operator struct {
	ctx     context.Context
	workers map[string]*Worker
	mu      sync.RWMutex
}

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
