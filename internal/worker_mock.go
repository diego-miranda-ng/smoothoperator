package internal

import (
	"context"

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
	HandleFunc         func(ctx context.Context, msg any) smoothoperator.HandleResult
	SetDispatcherFunc  func(disp smoothoperator.Dispatcher) // optional; if set, HandlerMock implements DispatcherAware
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
