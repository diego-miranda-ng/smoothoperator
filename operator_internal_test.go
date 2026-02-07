package smoothoperator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type noopHandler struct{}

func (noopHandler) Handle(context.Context, any) HandleResult { return Done() }

// quickHandler is a test handler that yields quickly (avoids importing internal and creating import cycle).
type quickHandler struct{ name string }

func (h quickHandler) Handle(ctx context.Context, msg any) HandleResult {
	select {
	case <-ctx.Done():
		return None(0)
	case <-time.After(10 * time.Millisecond):
		return Done()
	}
}

type idleHandler struct{ idle time.Duration }

func (h idleHandler) Handle(ctx context.Context, msg any) HandleResult {
	select {
	case <-ctx.Done():
		return None(0)
	default:
		return None(h.idle)
	}
}

type noneZeroHandler struct{}

func (noneZeroHandler) Handle(ctx context.Context, msg any) HandleResult { return None(0) }

type failHandler struct {
	err  error
	idle time.Duration
}

func (h failHandler) Handle(ctx context.Context, msg any) HandleResult {
	select {
	case <-ctx.Done():
		return Done()
	default:
		return Fail(h.err, h.idle)
	}
}

type panicHandler struct{ name string }

func (panicHandler) Handle(context.Context, any) HandleResult { panic("test panic") }

func TestWorkerStart_WhenAlreadyRunning_ShouldBeNoOp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w := newWorker("w", quickHandler{"w"}, Config{})
	require.NoError(t, w.Start(ctx))
	require.NoError(t, w.Start(ctx))
	<-w.Stop(ctx)
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorkerStop_WhenNotStarted_ShouldReturnClosedChannelImmediately(t *testing.T) {
	t.Parallel()
	w := newWorker("w", quickHandler{"w"}, Config{})
	ch := w.Stop(context.Background())
	require.NotNil(t, ch)
	_, open := <-ch
	require.False(t, open)
}

func TestWorker_WhenCreated_ShouldReturnNameAndStoppedStatus(t *testing.T) {
	t.Parallel()
	w := newWorker("my-name", quickHandler{"x"}, Config{})
	require.Equal(t, "my-name", w.getName())
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandleReturnsNone_ShouldSleepForIdleDuration(t *testing.T) {
	t.Parallel()
	w := newWorker("idle", idleHandler{15 * time.Millisecond}, Config{})
	ctx := context.Background()
	require.NoError(t, w.Start(ctx))
	time.Sleep(50 * time.Millisecond)
	<-w.Stop(ctx)
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorkerStop_WhenIdleSleep_ShouldCancelContextAndExitSelect(t *testing.T) {
	t.Parallel()
	w := newWorker("idle", idleHandler{5 * time.Second}, Config{})
	ctx := context.Background()
	require.NoError(t, w.Start(ctx))
	time.Sleep(50 * time.Millisecond)
	<-w.Stop(ctx)
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandleReturnsNoneWithZeroDuration_ShouldNotSleep(t *testing.T) {
	t.Parallel()
	w := newWorker("idle", noneZeroHandler{}, Config{})
	ctx := context.Background()
	require.NoError(t, w.Start(ctx))
	time.Sleep(30 * time.Millisecond)
	<-w.Stop(ctx)
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandleReturnsFail_ShouldLogErrorAndCanSleep(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("handler failed")
	w := newWorker("fail", failHandler{err: err, idle: 15 * time.Millisecond}, Config{})
	ctx := context.Background()
	require.NoError(t, w.Start(ctx))
	time.Sleep(50 * time.Millisecond)
	<-w.Stop(ctx)
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandlePanics_ShouldRecoverAndContinueUntilStop(t *testing.T) {
	t.Parallel()
	w := newWorker("panic-worker", panicHandler{}, Config{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, w.Start(ctx))
	time.Sleep(100 * time.Millisecond)
	stopChan := w.Stop(context.Background())
	select {
	case <-stopChan:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop within timeout")
	}
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenMaxPanicAttemptsReached_ShouldStop(t *testing.T) {
	t.Parallel()
	w := newWorker("panic-worker", panicHandler{}, Config{MaxPanicAttempts: 3})
	ctx := context.Background()
	require.NoError(t, w.Start(ctx))
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if w.getStatus() == StatusStopped {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.Equal(t, StatusStopped, w.getStatus())
}
