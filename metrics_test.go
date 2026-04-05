package smoothoperator_test

import (
	"context"
	"testing"
	"time"

	"github.com/diego-miranda-ng/smoothoperator"
	"github.com/diego-miranda-ng/smoothoperator/internal"
	"github.com/stretchr/testify/require"
)

func collectMetrics[T any](ch <-chan T) []T {
	var got []T
	for ev := range ch {
		got = append(got, ev)
	}
	return got
}

func TestMetricsRecorder_HandleMetrics_WhenChannelCreated_ReceivesHandleEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	w, err := op.AddHandler("worker-test", internal.NewHandlerMock(func(context.Context, any) smoothoperator.HandleResult {
		return smoothoperator.Done()
	}))
	require.NoError(t, err)

	ch := w.Metrics().HandleMetrics(10)

	// Act
	require.NoError(t, op.Start("worker-test"))
	time.Sleep(30 * time.Millisecond)
	<-op.StopAll()
	got := collectMetrics(ch)

	// Assert
	require.NotEmpty(t, got)
	require.Equal(t, smoothoperator.HandleStatusDone, got[0].Status)
	require.Equal(t, "worker-test", got[0].Worker)
}

func TestMetricsRecorder_LifecycleMetrics_WhenChannelCreated_ReceivesLifecycleEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	w, err := op.AddHandler("worker-test", internal.NewHandlerMock(func(context.Context, any) smoothoperator.HandleResult {
		return smoothoperator.Done()
	}))
	require.NoError(t, err)

	ch := w.Metrics().LifecycleMetrics(10)

	// Act
	require.NoError(t, op.Start("worker-test"))
	time.Sleep(30 * time.Millisecond)
	<-op.StopAll()
	got := collectMetrics(ch)

	// Assert
	require.GreaterOrEqual(t, len(got), 2)
	require.Equal(t, "started", got[0].Event)
	require.Equal(t, "stopped", got[len(got)-1].Event)
}

func TestMetricsRecorder_HandleMetrics_WithZeroBufferSize_CreatesChannel(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	w, err := op.AddHandler("worker-test", internal.NewHandlerMock(func(context.Context, any) smoothoperator.HandleResult {
		return smoothoperator.Done()
	}))
	require.NoError(t, err)

	// Act
	ch := w.Metrics().HandleMetrics(0)

	// Assert
	require.NotNil(t, ch)
}

func TestMetricsRecorder_PanicMetrics_WhenChannelCreated_ReceivesPanicEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	w, err := op.AddHandler(
		"worker-test",
		internal.PanicHandler(),
		smoothoperator.WithMaxPanicAttempts(1),
	)
	require.NoError(t, err)

	ch := w.Metrics().PanicMetrics(10)

	// Act
	require.NoError(t, op.Start("worker-test"))
	time.Sleep(30 * time.Millisecond)
	<-op.StopAll()
	got := collectMetrics(ch)

	// Assert
	require.NotEmpty(t, got)
	require.Equal(t, 1, got[0].Attempt)
	require.Equal(t, "worker-test", got[0].Worker)
	require.Contains(t, got[0].Err, "test panic")
}

func TestMetricsRecorder_DispatchMetrics_WhenChannelCreated_ReceivesDispatchEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	recordingHandler, _ := internal.NewRecordingHandler(5 * time.Second)
	w, err := op.AddHandler("worker-test", recordingHandler, smoothoperator.WithMessageOnly(true))
	require.NoError(t, err)

	ch := w.Metrics().DispatchMetrics(10)

	// Act
	require.NoError(t, op.Start("worker-test"))
	delivered, _, err := op.Dispatch(context.Background(), "worker-test", "hello")
	require.NoError(t, err)
	<-delivered
	<-op.StopAll()
	got := collectMetrics(ch)

	// Assert
	require.NotEmpty(t, got)
	require.True(t, got[0].Ok)
	require.Equal(t, "worker-test", got[0].Worker)
	require.Empty(t, got[0].Error)
}

func TestMetricsRecorder_HandleMetrics_WhenEventsExceedBufferSize_ShouldDropExtraEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	w, err := op.AddHandler("worker-test", internal.NoneZeroHandler())
	require.NoError(t, err)

	ch := w.Metrics().HandleMetrics(1)

	// Act
	require.NoError(t, op.Start("worker-test"))
	time.Sleep(30 * time.Millisecond)
	<-op.StopAll()
	got := collectMetrics(ch)

	// Assert
	require.Len(t, got, 1)
	require.Equal(t, smoothoperator.HandleStatusNone, got[0].Status)
	require.Equal(t, "worker-test", got[0].Worker)
}

func TestMetricsRecorder_PanicMetrics_WhenEventsExceedBufferSize_ShouldDropExtraEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	w, err := op.AddHandler(
		"worker-test",
		internal.PanicHandler(),
		smoothoperator.WithMaxPanicAttempts(3),
		smoothoperator.WithPanicBackoff(time.Millisecond),
	)
	require.NoError(t, err)

	ch := w.Metrics().PanicMetrics(1)

	// Act
	require.NoError(t, op.Start("worker-test"))
	time.Sleep(30 * time.Millisecond)
	<-op.StopAll()
	got := collectMetrics(ch)

	// Assert
	require.Len(t, got, 1)
	require.Equal(t, 1, got[0].Attempt)
	require.Equal(t, "worker-test", got[0].Worker)
	require.Contains(t, got[0].Err, "test panic")
}

func TestMetricsRecorder_DispatchMetrics_WhenEventsExceedBufferSize_ShouldDropExtraEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	w, err := op.AddHandler(
		"worker-test",
		internal.BlockingHandler(100*time.Millisecond),
		smoothoperator.WithMessageOnly(true),
		smoothoperator.WithMessageBufferSize(3),
	)
	require.NoError(t, err)

	ch := w.Metrics().DispatchMetrics(1)

	// Act
	require.NoError(t, op.Start("worker-test"))
	_, _, err = op.Dispatch(context.Background(), "worker-test", "first")
	require.NoError(t, err)
	_, _, err = op.Dispatch(context.Background(), "worker-test", "second")
	require.NoError(t, err)
	_, _, err = op.Dispatch(context.Background(), "worker-test", "third")
	require.NoError(t, err)
	<-op.StopAll()
	got := collectMetrics(ch)

	// Assert
	require.Len(t, got, 1)
	require.True(t, got[0].Ok)
	require.Equal(t, "worker-test", got[0].Worker)
	require.Empty(t, got[0].Error)
}

func TestMetricsRecorder_LifecycleMetrics_WhenEventsExceedBufferSize_ShouldDropExtraEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	w, err := op.AddHandler("worker-test", internal.NoneZeroHandler())
	require.NoError(t, err)

	ch := w.Metrics().LifecycleMetrics(1)

	// Act
	require.NoError(t, op.Start("worker-test"))
	time.Sleep(30 * time.Millisecond)
	<-op.StopAll()
	got := collectMetrics(ch)

	// Assert
	require.Len(t, got, 1)
	require.Equal(t, "started", got[0].Event)
	require.Equal(t, "worker-test", got[0].Worker)
}
