package smoothoperator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// handlerFunc is a Handler that delegates to fn; if Handle is called and fn is nil, it panics.
// Used in worker_test to avoid importing internal (which would create an import cycle).
type handlerFunc struct {
	fn func(context.Context, any) HandleResult
}

func (h handlerFunc) Handle(ctx context.Context, msg any) HandleResult {
	if h.fn == nil {
		panic("smoothoperator: handlerFunc.Handle not configured")
	}
	return h.fn(ctx, msg)
}

func TestWorkerStart_WhenAlreadyRunning_ShouldBeNoOp(t *testing.T) {
	t.Parallel()
	// Arrange
	quickHandle := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult {
		select {
		case <-ctx.Done():
			return None(0)
		case <-time.After(10 * time.Millisecond):
			return Done()
		}
	}}
	w := newWorker("w", quickHandle, applyHandlerOptions(), slog.Default().With("worker", "w"))
	ctx := context.Background()

	// Act
	require.NoError(t, w.start(ctx))
	require.NoError(t, w.start(ctx))
	<-w.stop()

	// Assert
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorkerStop_WhenNotStarted_ShouldReturnClosedChannelImmediately(t *testing.T) {
	t.Parallel()
	// Arrange
	quickHandle := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult { return Done() }}
	w := newWorker("w", quickHandle, applyHandlerOptions(), slog.Default().With("worker", "w"))

	// Act
	ch := w.stop()

	// Assert
	require.NotNil(t, ch)
	_, open := <-ch
	require.False(t, open)
}

func TestWorker_WhenCreated_ShouldReturnNameAndStoppedStatus(t *testing.T) {
	t.Parallel()
	// Arrange
	h := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult { return Done() }}
	w := newWorker("my-name", h, applyHandlerOptions(), slog.Default().With("worker", "my-name"))

	// Act
	name := w.getName()
	status := w.getStatus()

	// Assert
	require.Equal(t, "my-name", name)
	require.Equal(t, StatusStopped, status)
}

func TestWorker_WhenHandleReturnsNone_ShouldSleepForIdleDuration(t *testing.T) {
	t.Parallel()
	// Arrange
	idleDur := 15 * time.Millisecond
	h := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult {
		select {
		case <-ctx.Done():
			return None(0)
		default:
			return None(idleDur)
		}
	}}
	w := newWorker("idle", h, applyHandlerOptions(), slog.Default().With("worker", "idle"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond)

	// Act
	<-w.stop()

	// Assert
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorkerStop_WhenIdleSleep_ShouldCancelContextAndExitSelect(t *testing.T) {
	t.Parallel()
	// Arrange
	h := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult {
		select {
		case <-ctx.Done():
			return None(0)
		default:
			return None(5 * time.Second)
		}
	}}
	w := newWorker("idle", h, applyHandlerOptions(), slog.Default().With("worker", "idle"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond)

	// Act
	<-w.stop()

	// Assert
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandleReturnsNoneWithZeroDuration_ShouldNotSleep(t *testing.T) {
	t.Parallel()
	// Arrange
	h := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult {
		select {
		case <-ctx.Done():
			return Done()
		default:
			return None(0)
		}
	}}
	w := newWorker("idle", h, applyHandlerOptions(), slog.Default().With("worker", "idle"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(30 * time.Millisecond)

	// Act
	<-w.stop()

	// Assert
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandleReturnsFail_ShouldLogErrorAndCanSleep(t *testing.T) {
	t.Parallel()
	// Arrange
	handlerErr := fmt.Errorf("handler failed")
	idleDur := 15 * time.Millisecond
	h := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult {
		select {
		case <-ctx.Done():
			return Done()
		default:
			return Fail(handlerErr, idleDur)
		}
	}}
	w := newWorker("fail", h, applyHandlerOptions(), slog.Default().With("worker", "fail"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond)

	// Act
	<-w.stop()

	// Assert
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandlePanics_ShouldRecoverAndContinueUntilStop(t *testing.T) {
	t.Parallel()
	// Arrange
	h := handlerFunc{fn: func(context.Context, any) HandleResult { panic("test panic") }}
	w := newWorker("panic-worker", h, applyHandlerOptions(), slog.Default().With("worker", "panic-worker"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, w.start(ctx))
	time.Sleep(100 * time.Millisecond)

	// Act
	stopChan := w.stop()
	select {
	case <-stopChan:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop within timeout")
	}

	// Assert
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandlePanicsWithErrorValue_ShouldRecoverAndLogError(t *testing.T) {
	t.Parallel()
	// Arrange: panic with error value to cover panicToError's error branch
	panicErr := fmt.Errorf("handler panic error")
	h := handlerFunc{fn: func(context.Context, any) HandleResult { panic(panicErr) }}
	w := newWorker("panic-err", h, applyHandlerOptions(), slog.Default().With("worker", "panic-err"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond)

	// Act
	<-w.stop()

	// Assert
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenMaxPanicAttemptsReached_ShouldStop(t *testing.T) {
	t.Parallel()
	// Arrange
	h := handlerFunc{fn: func(context.Context, any) HandleResult { panic("test panic") }}
	w := newWorker("panic-worker", h, applyHandlerOptions(WithMaxPanicAttempts(3)), slog.Default().With("worker", "panic-worker"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))

	// Act
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if w.getStatus() == StatusStopped {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Assert
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenMessageSent_ShouldPassToHandler(t *testing.T) {
	t.Parallel()
	// Arrange
	var mu sync.Mutex
	var messages []any
	h := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult {
		if msg != nil {
			mu.Lock()
			messages = append(messages, msg)
			mu.Unlock()
			return Done()
		}
		select {
		case <-ctx.Done():
			return None(0)
		default:
			return None(5 * time.Second)
		}
	}}
	w := newWorker("w", h, applyHandlerOptions(), slog.Default().With("worker", "w"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond)

	env := envelope{msg: "direct-msg", delivered: make(chan struct{})}
	require.True(t, w.sendEnvelope(ctx, env))

	// Act
	select {
	case <-env.delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message not delivered within timeout")
	}
	<-w.stop()

	// Assert
	mu.Lock()
	msgs := append([]any(nil), messages...)
	mu.Unlock()
	require.Len(t, msgs, 1)
	require.Equal(t, "direct-msg", msgs[0])
}

func TestWorker_WhenIdleAndMessageSent_ShouldWakeUpImmediately(t *testing.T) {
	t.Parallel()
	// Arrange
	var mu sync.Mutex
	var messages []any
	h := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult {
		if msg != nil {
			mu.Lock()
			messages = append(messages, msg)
			mu.Unlock()
			return Done()
		}
		select {
		case <-ctx.Done():
			return None(0)
		default:
			return None(10 * time.Second)
		}
	}}
	w := newWorker("w", h, applyHandlerOptions(), slog.Default().With("worker", "w"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond)

	env := envelope{msg: "wake-msg", delivered: make(chan struct{})}
	start := time.Now()
	require.True(t, w.sendEnvelope(ctx, env))

	// Act
	select {
	case <-env.delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message not delivered; worker may not have woken from idle")
	}
	elapsed := time.Since(start)
	<-w.stop()

	// Assert
	require.Less(t, elapsed, 2*time.Second, "worker should wake from idle immediately on message")
	mu.Lock()
	msgs := append([]any(nil), messages...)
	mu.Unlock()
	require.Len(t, msgs, 1)
	require.Equal(t, "wake-msg", msgs[0])
}

func TestUnwrapEnvelope_WhenZeroValue_ShouldReturnNilWithoutPanic(t *testing.T) {
	t.Parallel()
	// Arrange
	h := handlerFunc{fn: func(context.Context, any) HandleResult { return Done() }}
	w := newWorker("w", h, applyHandlerOptions(), slog.Default().With("worker", "w"))

	// Act & Assert
	require.NotPanics(t, func() {
		msg := w.unwrapEnvelope(envelope{})
		require.Nil(t, msg)
	})
}

func TestUnwrapEnvelope_WhenValid_ShouldCloseDeliveredAndReturnMsg(t *testing.T) {
	t.Parallel()
	// Arrange
	h := handlerFunc{fn: func(context.Context, any) HandleResult { return Done() }}
	w := newWorker("w", h, applyHandlerOptions(), slog.Default().With("worker", "w"))
	delivered := make(chan struct{})
	env := envelope{msg: "payload", delivered: delivered}

	// Act
	msg := w.unwrapEnvelope(env)

	// Assert
	require.Equal(t, "payload", msg)
	select {
	case <-delivered:
	default:
		t.Fatal("delivered channel should have been closed")
	}
}

func TestWorker_WhenNoMessage_ShouldPassNilToHandler(t *testing.T) {
	t.Parallel()
	// Arrange
	var mu sync.Mutex
	var messages []any
	h := handlerFunc{fn: func(ctx context.Context, msg any) HandleResult {
		if msg != nil {
			mu.Lock()
			messages = append(messages, msg)
			mu.Unlock()
			return Done()
		}
		select {
		case <-ctx.Done():
			return None(0)
		default:
			return None(15 * time.Millisecond)
		}
	}}
	w := newWorker("w", h, applyHandlerOptions(), slog.Default().With("worker", "w"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(80 * time.Millisecond)

	// Act
	<-w.stop()

	// Assert
	mu.Lock()
	msgs := append([]any(nil), messages...)
	mu.Unlock()
	require.Empty(t, msgs)
}
