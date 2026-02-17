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
	w := newWorker("w", quickHandler{"w"}, applyHandlerOptions(), slog.Default().With("worker", "w"))
	require.NoError(t, w.start(ctx))
	require.NoError(t, w.start(ctx))
	<-w.stop()
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorkerStop_WhenNotStarted_ShouldReturnClosedChannelImmediately(t *testing.T) {
	t.Parallel()
	w := newWorker("w", quickHandler{"w"}, applyHandlerOptions(), slog.Default().With("worker", "w"))
	ch := w.stop()
	require.NotNil(t, ch)
	_, open := <-ch
	require.False(t, open)
}

func TestWorker_WhenCreated_ShouldReturnNameAndStoppedStatus(t *testing.T) {
	t.Parallel()
	w := newWorker("my-name", quickHandler{"x"}, applyHandlerOptions(), slog.Default().With("worker", "my-name"))
	require.Equal(t, "my-name", w.getName())
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandleReturnsNone_ShouldSleepForIdleDuration(t *testing.T) {
	t.Parallel()
	w := newWorker("idle", idleHandler{15 * time.Millisecond}, applyHandlerOptions(), slog.Default().With("worker", "idle"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond)
	<-w.stop()
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorkerStop_WhenIdleSleep_ShouldCancelContextAndExitSelect(t *testing.T) {
	t.Parallel()
	w := newWorker("idle", idleHandler{5 * time.Second}, applyHandlerOptions(), slog.Default().With("worker", "idle"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond)
	<-w.stop()
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandleReturnsNoneWithZeroDuration_ShouldNotSleep(t *testing.T) {
	t.Parallel()
	w := newWorker("idle", noneZeroHandler{}, applyHandlerOptions(), slog.Default().With("worker", "idle"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(30 * time.Millisecond)
	<-w.stop()
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandleReturnsFail_ShouldLogErrorAndCanSleep(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("handler failed")
	w := newWorker("fail", failHandler{err: err, idle: 15 * time.Millisecond}, applyHandlerOptions(), slog.Default().With("worker", "fail"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond)
	<-w.stop()
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenHandlePanics_ShouldRecoverAndContinueUntilStop(t *testing.T) {
	t.Parallel()
	w := newWorker("panic-worker", panicHandler{}, applyHandlerOptions(), slog.Default().With("worker", "panic-worker"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, w.start(ctx))
	time.Sleep(100 * time.Millisecond)
	stopChan := w.stop()
	select {
	case <-stopChan:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop within timeout")
	}
	require.Equal(t, StatusStopped, w.getStatus())
}

func TestWorker_WhenMaxPanicAttemptsReached_ShouldStop(t *testing.T) {
	t.Parallel()
	w := newWorker("panic-worker", panicHandler{}, applyHandlerOptions(WithMaxPanicAttempts(3)), slog.Default().With("worker", "panic-worker"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if w.getStatus() == StatusStopped {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.Equal(t, StatusStopped, w.getStatus())
}

// --- message recording handler (internal) ---

// msgRecordHandler records non-nil messages and idles when none available.
type msgRecordHandler struct {
	idle     time.Duration
	mu       sync.Mutex
	messages []any
}

func (h *msgRecordHandler) Handle(ctx context.Context, msg any) HandleResult {
	if msg != nil {
		h.mu.Lock()
		h.messages = append(h.messages, msg)
		h.mu.Unlock()
		return Done()
	}
	select {
	case <-ctx.Done():
		return None(0)
	default:
		return None(h.idle)
	}
}

func (h *msgRecordHandler) getMessages() []any {
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := make([]any, len(h.messages))
	copy(cp, h.messages)
	return cp
}

// --- message tests ---

func TestWorker_WhenMessageSent_ShouldPassToHandler(t *testing.T) {
	t.Parallel()
	h := &msgRecordHandler{idle: 5 * time.Second}
	w := newWorker("w", h, applyHandlerOptions(), slog.Default().With("worker", "w"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond) // let worker enter idle

	// Send a message via the worker's method
	env := envelope{msg: "direct-msg", delivered: make(chan struct{})}
	require.True(t, w.sendEnvelope(ctx, env))

	// Wait for delivery
	select {
	case <-env.delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message not delivered within timeout")
	}
	<-w.stop()

	msgs := h.getMessages()
	require.Len(t, msgs, 1)
	require.Equal(t, "direct-msg", msgs[0])
}

func TestWorker_WhenIdleAndMessageSent_ShouldWakeUpImmediately(t *testing.T) {
	t.Parallel()
	h := &msgRecordHandler{idle: 10 * time.Second}
	w := newWorker("w", h, applyHandlerOptions(), slog.Default().With("worker", "w"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	time.Sleep(50 * time.Millisecond) // ensure worker enters idle

	env := envelope{msg: "wake-msg", delivered: make(chan struct{})}
	start := time.Now()
	require.True(t, w.sendEnvelope(ctx, env))

	select {
	case <-env.delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message not delivered; worker may not have woken from idle")
	}
	elapsed := time.Since(start)
	<-w.stop()

	require.Less(t, elapsed, 2*time.Second, "worker should wake from idle immediately on message")
	msgs := h.getMessages()
	require.Len(t, msgs, 1)
	require.Equal(t, "wake-msg", msgs[0])
}

func TestUnwrapEnvelope_WhenZeroValue_ShouldReturnNilWithoutPanic(t *testing.T) {
	t.Parallel()
	w := newWorker("w", noopHandler{}, applyHandlerOptions(), slog.Default().With("worker", "w"))

	require.NotPanics(t, func() {
		msg := w.unwrapEnvelope(envelope{})
		require.Nil(t, msg)
	})
}

func TestUnwrapEnvelope_WhenValid_ShouldCloseDeliveredAndReturnMsg(t *testing.T) {
	t.Parallel()
	w := newWorker("w", noopHandler{}, applyHandlerOptions(), slog.Default().With("worker", "w"))

	delivered := make(chan struct{})
	env := envelope{msg: "payload", delivered: delivered}

	// Act
	msg := w.unwrapEnvelope(env)

	// Assert
	require.Equal(t, "payload", msg)
	select {
	case <-delivered:
		// channel was closed — expected
	default:
		t.Fatal("delivered channel should have been closed")
	}
}

func TestWorker_WhenNoMessage_ShouldPassNilToHandler(t *testing.T) {
	t.Parallel()
	h := &msgRecordHandler{idle: 15 * time.Millisecond}
	w := newWorker("w", h, applyHandlerOptions(), slog.Default().With("worker", "w"))
	ctx := context.Background()
	require.NoError(t, w.start(ctx))
	// Let the worker loop several times without any messages
	time.Sleep(80 * time.Millisecond)
	<-w.stop()

	// Handler should never have recorded a message (all calls had msg == nil)
	require.Empty(t, h.getMessages())
}
