package smoothoperator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type metricsTestHandler struct {
	fn func(context.Context, any) HandleResult
}

func (h metricsTestHandler) Handle(ctx context.Context, msg any) HandleResult {
	if h.fn != nil {
		return h.fn(ctx, msg)
	}
	return Done()
}

func TestMetricsRecorder_LastMetric_WhenNoEvents_ReturnsFalse(t *testing.T) {
	t.Parallel()
	// Arrange
	op := NewOperator(context.Background())
	_, err := op.AddHandler("w", metricsTestHandler{})
	require.NoError(t, err)
	w, err := op.Worker("w")
	require.NoError(t, err)

	// Act
	_, ok := w.LastMetric()

	// Assert
	require.False(t, ok)
}

func TestMetricsRecorder_LastMetric_WhenWorkerRan_ReturnsLatestEvent(t *testing.T) {
	t.Parallel()
	// Arrange
	op := NewOperator(context.Background())
	_, err := op.AddHandler("w", metricsTestHandler{})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond)
	defer func() { <-op.StopAll() }()

	w, err := op.Worker("w")
	require.NoError(t, err)

	// Act
	ev, ok := w.LastMetric()

	// Assert
	require.True(t, ok)
	require.Equal(t, MetricKindHandle, ev.Kind)
	require.Equal(t, "w", ev.Worker)
	require.Equal(t, HandleStatusDone, ev.Status)
}

func TestMetricsRecorder_Metrics_WhenChannelCreated_ReturnsChannelThatReceivesEvents(t *testing.T) {
	t.Parallel()
	// Arrange
	op := NewOperator(context.Background())
	_, err := op.AddHandler("w", metricsTestHandler{})
	require.NoError(t, err)
	w, err := op.Worker("w")
	require.NoError(t, err)
	ch := w.Metrics(10)
	var got []MetricEvent
	done := make(chan struct{})
	go func() {
		for ev := range ch {
			got = append(got, ev)
		}
		close(done)
	}()
	require.NoError(t, op.Start("w"))
	time.Sleep(30 * time.Millisecond)

	// Act
	<-op.StopAll()
	<-done

	// Assert
	require.NotEmpty(t, got)
	var hasLifecycle, hasHandle bool
	for _, ev := range got {
		if ev.Kind == MetricKindLifecycle {
			hasLifecycle = true
		}
		if ev.Kind == MetricKindHandle {
			hasHandle = true
		}
	}
	require.True(t, hasLifecycle)
	require.True(t, hasHandle)
}

func TestMetricsRecorder_Metrics_WithZeroBufferSize_CreatesChannel(t *testing.T) {
	t.Parallel()
	// Arrange
	op := NewOperator(context.Background())
	_, err := op.AddHandler("w", metricsTestHandler{})
	require.NoError(t, err)
	w, err := op.Worker("w")
	require.NoError(t, err)
	ch := w.Metrics(0)
	// Consume in background so worker is not blocked when sending metrics
	go func() { for range ch {} }()

	// Act
	require.NoError(t, op.Start("w"))
	time.Sleep(20 * time.Millisecond)
	<-op.StopAll()

	// Assert
	require.NotNil(t, ch)
}
