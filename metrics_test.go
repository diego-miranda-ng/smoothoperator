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

	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.NewHandlerMock(func(context.Context, any) smoothoperator.HandleResult {
		return smoothoperator.Done()
	}))
	require.NoError(t, err)
	w, err := op.Worker("w")
	require.NoError(t, err)
	ch := w.HandleMetrics(10)
	var got []smoothoperator.HandleMetricEvent
	done := make(chan struct{})
	go func() {
		for ev := range ch {
			got = append(got, ev)
		}
		close(done)
	}()
	require.NoError(t, op.Start("w"))
	time.Sleep(30 * time.Millisecond)

	<-op.StopAll()
	<-done

	require.NotEmpty(t, got)
	require.Equal(t, smoothoperator.HandleStatusDone, got[0].Status)
	require.Equal(t, "w", got[0].Worker)
}

func TestMetricsRecorder_LifecycleMetrics_WhenChannelCreated_ReceivesLifecycleEvents(t *testing.T) {
	t.Parallel()

	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.NewHandlerMock(func(context.Context, any) smoothoperator.HandleResult {
		return smoothoperator.Done()
	}))
	require.NoError(t, err)
	w, err := op.Worker("w")
	require.NoError(t, err)
	ch := w.LifecycleMetrics(10)
	var got []smoothoperator.LifecycleMetricEvent
	done := make(chan struct{})
	go func() {
		for ev := range ch {
			got = append(got, ev)
		}
		close(done)
	}()
	require.NoError(t, op.Start("w"))
	time.Sleep(30 * time.Millisecond)

	<-op.StopAll()
	<-done

	require.GreaterOrEqual(t, len(got), 2)
	require.Equal(t, "started", got[0].Event)
	require.Equal(t, "stopped", got[len(got)-1].Event)
}

func TestMetricsRecorder_HandleMetrics_WithZeroBufferSize_CreatesChannel(t *testing.T) {
	t.Parallel()

	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.NewHandlerMock(func(context.Context, any) smoothoperator.HandleResult {
		return smoothoperator.Done()
	}))
	require.NoError(t, err)
	w, err := op.Worker("w")
	require.NoError(t, err)
	ch := w.HandleMetrics(0)
	go func() {
		for range ch {
		}
	}()

	require.NoError(t, op.Start("w"))
	time.Sleep(20 * time.Millisecond)
	<-op.StopAll()

	require.NotNil(t, ch)
}
