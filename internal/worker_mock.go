package internal

import (
	"context"
	"fmt"
	"time"
	"github.com/diego-miranda-ng/smoothoperator"
)

// handlerMock is a Handler implementation for testing.
type handlerMock struct {
	name string
}

// NewHandler returns a Handler that logs and simulates work each Handle call.
func NewHandler(name string) workermanager.Handler {
	return &handlerMock{name: name}
}

func (h *handlerMock) Handle(ctx context.Context) workermanager.HandleResult {
	fmt.Println("worker", h.name, " is working")
	select {
	case <-ctx.Done():
		return workermanager.None(0)
	case <-time.After(5 * time.Second):
		return workermanager.Done()
	}
}

// QuickHandler returns a Handler that yields quickly (for tests). Handle exits
// after a short delay or on ctx.Done(), so Stop() completes fast.
func QuickHandler(name string) workermanager.Handler {
	return &quickHandler{name: name}
}

type quickHandler struct {
	name string
}

func (h *quickHandler) Handle(ctx context.Context) workermanager.HandleResult {
	select {
	case <-ctx.Done():
		return workermanager.None(0)
	case <-time.After(10 * time.Millisecond):
		return workermanager.Done()
	}
}

// IdleHandler returns a Handler that always reports no work and asks to sleep.
// Used to cover None() and the idle-sleep branch in the worker.
func IdleHandler(name string, idle time.Duration) workermanager.Handler {
	return &idleHandler{name: name, idle: idle}
}

type idleHandler struct {
	name string
	idle time.Duration
}

func (h *idleHandler) Handle(ctx context.Context) workermanager.HandleResult {
	select {
	case <-ctx.Done():
		return workermanager.None(0)
	default:
		return workermanager.None(h.idle)
	}
}

// FailHandler returns a Handler that returns Fail with the given error and idle duration.
// Used to cover Fail() and the error path in the worker.
func FailHandler(name string, err error, idle time.Duration) workermanager.Handler {
	return &failHandler{name: name, err: err, idle: idle}
}

type failHandler struct {
	name string
	err  error
	idle time.Duration
}

func (h *failHandler) Handle(ctx context.Context) workermanager.HandleResult {
	select {
	case <-ctx.Done():
		return workermanager.Done()
	default:
		return workermanager.Fail(h.err, h.idle)
	}
}

// NoneZeroHandler returns a Handler that returns None(0) so worker does not sleep (covers IdleDuration==0 path).
func NoneZeroHandler(name string) workermanager.Handler {
	return &noneZeroHandler{name: name}
}

type noneZeroHandler struct {
	name string
}

func (h *noneZeroHandler) Handle(ctx context.Context) workermanager.HandleResult {
	select {
	case <-ctx.Done():
		return workermanager.Done()
	default:
		return workermanager.None(0)
	}
}
