package smoothoperator_test

import (
	"context"
	"testing"
	"time"

	"github.com/diego-miranda-ng/smoothoperator"
	"github.com/diego-miranda-ng/smoothoperator/internal"
	"github.com/stretchr/testify/require"
)

func TestMetricsRecorder_HandleMetrics_WhenChannelCreated_ReceivesHandleEvents(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	w, err := op.AddHandler("worker-test", internal.NewHandlerMock(func(context.Context, any) smoothoperator.HandleResult {
		return smoothoperator.Done()
	}))
	require.NoError(t, err)

	ch := w.Metrics().HandleMetrics(10)
	var got []smoothoperator.HandleMetricEvent
	done := make(chan struct{})
	go func() {
		for ev := range ch {
			got = append(got, ev)
		}
		close(done)
	}()

	// Act
	require.NoError(t, op.Start("worker-test"))
	time.Sleep(30 * time.Millisecond)
	<-op.StopAll()
	<-done

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
	var got []smoothoperator.LifecycleMetricEvent
	done := make(chan struct{})
	go func() {
		for ev := range ch {
			got = append(got, ev)
		}
		close(done)
	}()

	// Act
	require.NoError(t, op.Start("worker-test"))
	time.Sleep(30 * time.Millisecond)
	<-op.StopAll()
	<-done

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
