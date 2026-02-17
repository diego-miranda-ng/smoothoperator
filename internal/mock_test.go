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

func TestWorkerMock_Metrics_WhenMetricsFuncNil_Panics(t *testing.T) {
	t.Parallel()
	// Arrange
	m := &WorkerMock{LastMetricFunc: func() (smoothoperator.MetricEvent, bool) { return smoothoperator.MetricEvent{}, false }}

	// Act & Assert
	require.PanicsWithValue(t, errWorkerMockMetricsNotConfigured, func() {
		_ = m.Metrics(1)
	})
}

func TestWorkerMock_LastMetric_WhenLastMetricFuncNil_Panics(t *testing.T) {
	t.Parallel()
	// Arrange
	m := &WorkerMock{MetricsFunc: func(int) <-chan smoothoperator.MetricEvent { return nil }}

	// Act & Assert
	require.PanicsWithValue(t, errWorkerMockLastMetricNotConfigured, func() {
		_, _ = m.LastMetric()
	})
}
