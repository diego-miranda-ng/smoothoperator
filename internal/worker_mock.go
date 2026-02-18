package internal

import (
	"context"
	"sync"
	"time"

	"github.com/diego-miranda-ng/smoothoperator"
)

const errHandlerMockNotConfigured = "smoothoperator/internal: HandlerMock.Handle not configured"
const errDispatcherMockNotConfigured = "smoothoperator/internal: DispatcherMock.Dispatch not configured"
const errWorkerMockMetricsNotConfigured = "smoothoperator/internal: WorkerMock.Metrics not configured"
const errWorkerMockLastMetricNotConfigured = "smoothoperator/internal: WorkerMock.LastMetric not configured"

// HandlerMock implements Handler. It delegates to HandleFunc; if Handle is
// called and HandleFunc is nil, it panics. Optionally implements
// DispatcherAware via SetDispatcherFunc.
type HandlerMock struct {
	HandleFunc        func(ctx context.Context, msg any) smoothoperator.HandleResult
	SetDispatcherFunc func(disp smoothoperator.Dispatcher) // optional; if set, HandlerMock implements DispatcherAware
}

// NewHandlerMock returns a HandlerMock that uses fn for Handle. If fn is nil,
// Handle will panic when called. Tests must pass a non-nil function.
func NewHandlerMock(fn func(context.Context, any) smoothoperator.HandleResult) *HandlerMock {
	return &HandlerMock{HandleFunc: fn}
}

// Handle implements Handler. Panics if HandleFunc is nil.
func (m *HandlerMock) Handle(ctx context.Context, msg any) smoothoperator.HandleResult {
	if m.HandleFunc == nil {
		panic(errHandlerMockNotConfigured)
	}
	return m.HandleFunc(ctx, msg)
}

// SetDispatcher implements DispatcherAware. No-op if SetDispatcherFunc is nil
// (so handlers that only need Handle do not have to set it when used with AddHandler).
// Panics only when the test explicitly needs dispatcher behavior but did not set SetDispatcherFunc;
// for normal Handler-only tests, AddHandler may call SetDispatcher and it is a no-op.
func (m *HandlerMock) SetDispatcher(disp smoothoperator.Dispatcher) {
	if m.SetDispatcherFunc != nil {
		m.SetDispatcherFunc(disp)
	}
}

// DispatcherMock implements Dispatcher. It delegates to DispatchFunc; if
// Dispatch is called and DispatchFunc is nil, it panics.
type DispatcherMock struct {
	DispatchFunc func(ctx context.Context, name string, msg any) (<-chan struct{}, <-chan any, error)
}

// NewDispatcherMock returns a DispatcherMock that uses fn for Dispatch. If fn
// is nil, Dispatch will panic when called.
func NewDispatcherMock(fn func(context.Context, string, any) (<-chan struct{}, <-chan any, error)) *DispatcherMock {
	return &DispatcherMock{DispatchFunc: fn}
}

// Dispatch implements Dispatcher. Panics if DispatchFunc is nil.
func (m *DispatcherMock) Dispatch(ctx context.Context, name string, msg any) (<-chan struct{}, <-chan any, error) {
	if m.DispatchFunc == nil {
		panic(errDispatcherMockNotConfigured)
	}
	return m.DispatchFunc(ctx, name, msg)
}

// WorkerMock implements Worker. It delegates to MetricsFunc and
// LastMetricFunc; if either method is called with its func nil, it panics.
type WorkerMock struct {
	MetricsFunc    func(bufferSize int) <-chan smoothoperator.MetricEvent
	LastMetricFunc func() (smoothoperator.MetricEvent, bool)
}

// NewWorkerMock returns a WorkerMock. Callers must set MetricsFunc and
// LastMetricFunc before calling Metrics or LastMetric, or those calls will panic.
func NewWorkerMock(metricsFn func(int) <-chan smoothoperator.MetricEvent, lastMetricFn func() (smoothoperator.MetricEvent, bool)) *WorkerMock {
	return &WorkerMock{MetricsFunc: metricsFn, LastMetricFunc: lastMetricFn}
}

// Metrics implements Worker. Panics if MetricsFunc is nil.
func (m *WorkerMock) Metrics(bufferSize int) <-chan smoothoperator.MetricEvent {
	if m.MetricsFunc == nil {
		panic(errWorkerMockMetricsNotConfigured)
	}
	return m.MetricsFunc(bufferSize)
}

// LastMetric implements Worker. Panics if LastMetricFunc is nil.
func (m *WorkerMock) LastMetric() (smoothoperator.MetricEvent, bool) {
	if m.LastMetricFunc == nil {
		panic(errWorkerMockLastMetricNotConfigured)
	}
	return m.LastMetricFunc()
}

// NewRecordingHandler returns a Handler (using HandlerMock) that records non-nil
// messages and a function to retrieve them. The handler idles for idle when msg is nil.
// Use from tests that need to assert on messages received by the handler.
func NewRecordingHandler(idle time.Duration) (smoothoperator.Handler, func() []any) {
	var mu sync.Mutex
	var messages []any
	h := NewHandlerMock(func(ctx context.Context, msg any) smoothoperator.HandleResult {
		if msg != nil {
			mu.Lock()
			messages = append(messages, msg)
			mu.Unlock()
			return smoothoperator.Done()
		}
		select {
		case <-ctx.Done():
			return smoothoperator.None(0)
		default:
			return smoothoperator.None(idle)
		}
	})

	getMessages := func() []any {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]any, len(messages))
		copy(cp, messages)
		return cp
	}
	return h, getMessages
}

// QuickHandler returns a Handler that yields after 10ms or on ctx.Done (for fast tests).
func QuickHandler() smoothoperator.Handler {
	return NewHandlerMock(func(ctx context.Context, msg any) smoothoperator.HandleResult {
		select {
		case <-ctx.Done():
			return smoothoperator.None(0)
		case <-time.After(10 * time.Millisecond):
			return smoothoperator.Done()
		}
	})
}

// IdleHandler returns a Handler that reports no work and idles for the given duration.
func IdleHandler(idle time.Duration) smoothoperator.Handler {
	return NewHandlerMock(func(ctx context.Context, msg any) smoothoperator.HandleResult {
		select {
		case <-ctx.Done():
			return smoothoperator.None(0)
		default:
			return smoothoperator.None(idle)
		}
	})
}

// NoneZeroHandler returns a Handler that returns None(0) so the worker does not sleep.
func NoneZeroHandler() smoothoperator.Handler {
	return NewHandlerMock(func(ctx context.Context, msg any) smoothoperator.HandleResult {
		select {
		case <-ctx.Done():
			return smoothoperator.Done()
		default:
			return smoothoperator.None(0)
		}
	})
}

// FailHandler returns a Handler that returns Fail with the given error and idle duration.
func FailHandler(err error, idle time.Duration) smoothoperator.Handler {
	return NewHandlerMock(func(ctx context.Context, msg any) smoothoperator.HandleResult {
		select {
		case <-ctx.Done():
			return smoothoperator.Done()
		default:
			return smoothoperator.Fail(err, idle)
		}
	})
}

// PanicHandler returns a Handler that panics on every Handle call.
func PanicHandler() smoothoperator.Handler {
	return NewHandlerMock(func(context.Context, any) smoothoperator.HandleResult { panic("test panic") })
}

// ResultHandler returns a Handler that, when it receives a non-nil message, returns DoneWithResult(result); when msg is nil it idles.
func ResultHandler(idle time.Duration, result any) smoothoperator.Handler {
	return NewHandlerMock(func(ctx context.Context, msg any) smoothoperator.HandleResult {
		if msg != nil {
			return smoothoperator.DoneWithResult(result)
		}
		select {
		case <-ctx.Done():
			return smoothoperator.None(0)
		default:
			return smoothoperator.None(idle)
		}
	})
}

// BlockingHandler returns a Handler that blocks in Handle for the given duration (or until ctx is done), then returns Done.
func BlockingHandler(blockFor time.Duration) smoothoperator.Handler {
	return NewHandlerMock(func(ctx context.Context, msg any) smoothoperator.HandleResult {
		select {
		case <-ctx.Done():
			return smoothoperator.Done()
		case <-time.After(blockFor):
			return smoothoperator.Done()
		}
	})
}

// ForwarderHandler returns a Handler that forwards non-nil messages to target via Dispatcher (implements DispatcherAware).
func ForwarderHandler(target string, idle time.Duration) smoothoperator.Handler {
	var disp smoothoperator.Dispatcher
	m := &HandlerMock{}
	m.SetDispatcherFunc = func(d smoothoperator.Dispatcher) { disp = d }
	m.HandleFunc = func(ctx context.Context, msg any) smoothoperator.HandleResult {
		if msg != nil && disp != nil {
			_, _, _ = disp.Dispatch(ctx, target, msg)
		}
		return smoothoperator.None(idle)
	}
	return m
}
