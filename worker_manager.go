package workermanager

import (
	"context"
	"fmt"
	"sync"
)

type WorkManager interface {
	AddHandler(name string, handler Handler) (*Worker, error)
	Start(name string) error
	StartAll() error
	Stop(name string) (chan struct{}, error)
	StopAll() chan struct{}
}

type workerManager struct {
	ctx     context.Context
	workers map[string]*Worker
	mu      sync.RWMutex
}

func NewWorkerManager(ctx context.Context) WorkManager {
	return &workerManager{
		ctx:     ctx,
		workers: make(map[string]*Worker),
		mu:      sync.RWMutex{},
	}
}

func (wm *workerManager) AddHandler(name string, handler Handler) (*Worker, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, ok := wm.workers[name]; ok {
		return nil, fmt.Errorf("worker %s already exists", name)
	}

	worker := NewWorker(name, handler)
	wm.workers[name] = worker

	return worker, nil
}

func (wm *workerManager) Start(name string) error {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	worker, ok := wm.workers[name]
	if !ok {
		return fmt.Errorf("worker %s not found", name)
	}

	return worker.Start(wm.ctx)
}

func (wm *workerManager) StartAll() error {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	for _, worker := range wm.workers {
		if err := wm.Start(worker.Name()); err != nil {
			return err
		}
	}
	return nil
}

func (wm *workerManager) Stop(name string) (chan struct{}, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	worker, ok := wm.workers[name]
	if !ok {
		return nil, fmt.Errorf("worker %s not found", name)
	}

	return worker.Stop(context.Background()), nil
}

func (wm *workerManager) StopAll() chan struct{} {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	for _, worker := range wm.workers {
		if stopChan, _ := wm.Stop(worker.Name()); stopChan != nil {
			<-stopChan
		}
	}

	stopped := make(chan struct{})
	close(stopped)
	return stopped
}
