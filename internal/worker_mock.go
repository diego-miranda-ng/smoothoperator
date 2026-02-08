package internal

import (
	"context"
	"fmt"
	"sync"
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

func (h *handlerMock) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
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

func (h *quickHandler) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
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

func (h *idleHandler) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
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

func (h *failHandler) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
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

func (h *noneZeroHandler) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
	select {
	case <-ctx.Done():
		return smoothoperator.Done()
	default:
		return smoothoperator.None(0)
	}
}

// MessageRecorder is a Handler that records non-nil messages received via Handle
// and idles when no message is present. Use Messages() to inspect received messages.
type MessageRecorder struct {
	idle     time.Duration
	mu       sync.Mutex
	messages []any
}

// NewMessageRecorder returns a MessageRecorder that idles for the given duration
// when no message is available.
func NewMessageRecorder(idle time.Duration) *MessageRecorder {
	return &MessageRecorder{idle: idle}
}

func (h *MessageRecorder) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
	if msg != nil {
		h.mu.Lock()
		h.messages = append(h.messages, msg)
		h.mu.Unlock()
		return smoothoperator.Done()
	}
	select {
	case <-ctx.Done():
		return smoothoperator.None(0)
	default:
		return smoothoperator.None(h.idle)
	}
}

// Messages returns a copy of all non-nil messages received by Handle.
func (h *MessageRecorder) Messages() []any {
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := make([]any, len(h.messages))
	copy(cp, h.messages)
	return cp
}

// ResultHandler returns a Handler that, when it receives a non-nil message,
// returns DoneWithResult with the given result. When msg is nil it idles.
// Used to test the result channel returned by Dispatch.
func ResultHandler(idle time.Duration, result any) smoothoperator.Handler {
	return &resultHandler{idle: idle, result: result}
}

type resultHandler struct {
	idle   time.Duration
	result any
}

func (h *resultHandler) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
	if msg != nil {
		return smoothoperator.DoneWithResult(h.result)
	}
	select {
	case <-ctx.Done():
		return smoothoperator.None(0)
	default:
		return smoothoperator.None(h.idle)
	}
}

// ForwarderHandler returns a Handler that forwards any non-nil message to the
// worker named target via the Dispatcher. It implements DispatcherAware to
// receive the dispatcher at registration. Used to test handler-to-handler messaging.
func ForwarderHandler(target string, idle time.Duration) smoothoperator.Handler {
	return &forwarderHandler{target: target, idle: idle}
}

type forwarderHandler struct {
	target     string
	idle       time.Duration
	dispatcher smoothoperator.Dispatcher
}

func (h *forwarderHandler) SetDispatcher(disp smoothoperator.Dispatcher) { h.dispatcher = disp }

func (h *forwarderHandler) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
	if msg != nil && h.dispatcher != nil {
		_, _, _ = h.dispatcher.Dispatch(h.target, msg)
		return smoothoperator.Done()
	}
	select {
	case <-ctx.Done():
		return smoothoperator.None(0)
	default:
		return smoothoperator.None(h.idle)
	}
}

// PanicHandler returns a Handler that panics on every Handle call. Used to test panic recovery.
func PanicHandler(name string) smoothoperator.Handler {
	return &panicHandler{name: name}
}

type panicHandler struct {
	name string
}

func (h *panicHandler) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
	panic("panic from handler: " + h.name)
}
