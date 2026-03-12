package internal

import (
	"context"
	"testing"

	"github.com/diego-miranda-ng/smoothoperator"
	"github.com/stretchr/testify/require"
)

func TestHandlerMock_Handle_WhenHandleFuncNil_Panics(t *testing.T) {
	t.Parallel()
	// Arrange
	m := NewHandlerMock(nil)

	// Act & Assert
	require.PanicsWithValue(t, errHandlerMockNotConfigured, func() {
		m.Handle(context.Background(), nil)
	})
}

func TestHandlerMock_SetDispatcher_WhenSetDispatcherFuncNil_NoOp(t *testing.T) {
	t.Parallel()
	// Arrange
	m := &HandlerMock{HandleFunc: func(context.Context, any) smoothoperator.HandleResult { return smoothoperator.Done() }}

	// Act & Assert: does not panic (no-op when not configured)
	require.NotPanics(t, func() { m.SetDispatcher(nil) })
}

func TestDispatcherMock_Dispatch_WhenDispatchFuncNil_Panics(t *testing.T) {
	t.Parallel()
	// Arrange
	m := NewDispatcherMock(nil)

	// Act & Assert
	require.PanicsWithValue(t, errDispatcherMockNotConfigured, func() {
		_, _, _ = m.Dispatch(context.Background(), "w", nil)
	})
}

func TestWorkerMock_HandleMetrics_WhenFuncNil_Panics(t *testing.T) {
	t.Parallel()
	m := &WorkerMock{}
	require.PanicsWithValue(t, errWorkerMockHandleMetricsNotConfigured, func() {
		_ = m.HandleMetrics(1)
	})
}

func TestWorkerMock_PanicMetrics_WhenFuncNil_Panics(t *testing.T) {
	t.Parallel()
	m := &WorkerMock{}
	require.PanicsWithValue(t, errWorkerMockPanicMetricsNotConfigured, func() {
		_ = m.PanicMetrics(1)
	})
}

func TestWorkerMock_DispatchMetrics_WhenFuncNil_Panics(t *testing.T) {
	t.Parallel()
	m := &WorkerMock{}
	require.PanicsWithValue(t, errWorkerMockDispatchMetricsNotConfigured, func() {
		_ = m.DispatchMetrics(1)
	})
}

func TestWorkerMock_LifecycleMetrics_WhenFuncNil_Panics(t *testing.T) {
	t.Parallel()
	m := &WorkerMock{}
	require.PanicsWithValue(t, errWorkerMockLifecycleMetricsNotConfigured, func() {
		_ = m.LifecycleMetrics(1)
	})
}
