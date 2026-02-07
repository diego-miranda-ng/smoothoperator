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
func NewHandler(name string) smoothoperator.Handler {
	return &handlerMock{name: name}
}

func (h *handlerMock) Handle(ctx context.Context) smoothoperator.HandleResult {
	fmt.Println("worker", h.name, " is working")
	select {
	case <-ctx.Done():
		return smoothoperator.None(0)
	case <-time.After(5 * time.Second):
		return smoothoperator.Done()
	}
}

// QuickHandler returns a Handler that yields quickly (for tests). Handle exits
// after a short delay or on ctx.Done(), so Stop() completes fast.
func QuickHandler(name string) smoothoperator.Handler {
	return &quickHandler{name: name}
}

type quickHandler struct {
	name string
}

func (h *quickHandler) Handle(ctx context.Context) smoothoperator.HandleResult {
	select {
	case <-ctx.Done():
		return smoothoperator.None(0)
	case <-time.After(10 * time.Millisecond):
		return smoothoperator.Done()
	}
}

// IdleHandler returns a Handler that always reports no work and asks to sleep.
// Used to cover None() and the idle-sleep branch in the worker.
func IdleHandler(name string, idle time.Duration) smoothoperator.Handler {
	return &idleHandler{name: name, idle: idle}
}

type idleHandler struct {
	name string
	idle time.Duration
}

func (h *idleHandler) Handle(ctx context.Context) smoothoperator.HandleResult {
	select {
	case <-ctx.Done():
		return smoothoperator.None(0)
	default:
		return smoothoperator.None(h.idle)
	}
}

// FailHandler returns a Handler that returns Fail with the given error and idle duration.
// Used to cover Fail() and the error path in the worker.
func FailHandler(name string, err error, idle time.Duration) smoothoperator.Handler {
	return &failHandler{name: name, err: err, idle: idle}
}

type failHandler struct {
	name string
	err  error
	idle time.Duration
}

func (h *failHandler) Handle(ctx context.Context) smoothoperator.HandleResult {
	select {
	case <-ctx.Done():
		return smoothoperator.Done()
	default:
		return smoothoperator.Fail(h.err, h.idle)
	}
}

// NoneZeroHandler returns a Handler that returns None(0) so worker does not sleep (covers IdleDuration==0 path).
func NoneZeroHandler(name string) smoothoperator.Handler {
	return &noneZeroHandler{name: name}
}

type noneZeroHandler struct {
	name string
}

func (h *noneZeroHandler) Handle(ctx context.Context) smoothoperator.HandleResult {
	select {
	case <-ctx.Done():
		return smoothoperator.Done()
	default:
		return smoothoperator.None(0)
	}
}

// PanicHandler returns a Handler that panics on every Handle call. Used to test panic recovery.
func PanicHandler(name string) smoothoperator.Handler {
	return &panicHandler{name: name}
}

type panicHandler struct {
	name string
}

func (h *panicHandler) Handle(ctx context.Context) smoothoperator.HandleResult {
	panic("panic from handler: " + h.name)
}
