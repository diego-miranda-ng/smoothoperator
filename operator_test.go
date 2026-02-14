package smoothoperator_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/diego-miranda-ng/smoothoperator"
	"github.com/diego-miranda-ng/smoothoperator/internal"

	"github.com/stretchr/testify/require"
)

func TestStopAll_WhenMultipleWorkersRunning_ShouldWaitAllWorkersToStop(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	names := []string{"worker-1", "worker-2", "worker-3", "worker-4", "worker-5"}
	for _, name := range names {
		_, err := op.AddHandler(name, internal.QuickHandler(name), smoothoperator.Config{})
		require.NoError(t, err)
		op.Start(name)
	}
	time.Sleep(20 * time.Millisecond)

	// Act
	<-op.StopAll()

	// Assert
	for _, name := range names {
		status, err := op.Status(name)
		require.NoError(t, err)
		require.Equal(t, smoothoperator.StatusStopped, status)
	}
}

func TestStop_WhenWorkerRunning_ShouldAwaitWorkerStop(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("worker-1", internal.QuickHandler("worker-1"), smoothoperator.Config{})
	require.NoError(t, err)
	op.Start("worker-1")
	time.Sleep(20 * time.Millisecond)

	// Act
	stopChan, err := op.Stop("worker-1")
	require.NoError(t, err)
	<-stopChan

	// Assert
	status, err := op.Status("worker-1")
	require.NoError(t, err)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestAddHandler_WhenDuplicateName_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())

	// Act
	_, err := op.AddHandler("a", internal.QuickHandler("a"), smoothoperator.Config{})
	require.NoError(t, err)
	w, err := op.AddHandler("a", internal.QuickHandler("a"), smoothoperator.Config{})
	require.Error(t, err)
	require.Nil(t, w, "worker should be nil when error occurs")

	// Assert
	require.Contains(t, err.Error(), "already exists")
}

func TestStart_WhenWorkerNotFound_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())

	// Act
	err := op.Start("missing")
	require.Error(t, err)

	// Assert
	require.Contains(t, err.Error(), "not found")
}

func TestStartAll_WhenWorkersAdded_ShouldStartAllWorkers(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w1", internal.QuickHandler("w1"), smoothoperator.Config{})
	require.NoError(t, err)
	_, err = op.AddHandler("w2", internal.QuickHandler("w2"), smoothoperator.Config{})
	require.NoError(t, err)

	// Act
	require.NoError(t, op.StartAll())
	time.Sleep(20 * time.Millisecond)
	<-op.StopAll()

	// Assert
	// (implicit: StopAll completes without hanging)
}

func TestStop_WhenWorkerNotFound_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())

	// Act
	ch, err := op.Stop("missing")

	// Assert
	require.Error(t, err)
	require.Nil(t, ch)
	require.Contains(t, err.Error(), "not found")
}

func TestWorkerStart_WhenAlreadyRunning_ShouldBeNoOp(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.QuickHandler("w"), smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))

	// Act: second start is no-op
	require.NoError(t, op.Start("w"))
	<-op.StopAll()

	// Assert
	status, err := op.Status("w")
	require.NoError(t, err)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestWorkerStop_WhenNotStarted_ShouldReturnClosedChannelImmediately(t *testing.T) {
	t.Parallel()

	// Arrange: add handler but never start it
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.QuickHandler("w"), smoothoperator.Config{})
	require.NoError(t, err)

	// Act
	ch, err := op.Stop("w")
	require.NoError(t, err)

	// Assert: channel closes immediately
	require.NotNil(t, ch)
	_, open := <-ch
	require.False(t, open, "channel should be closed immediately when worker was never started")
}

func TestWorker_WhenCreated_ShouldReturnNameAndStoppedStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("my-name", internal.QuickHandler("x"), smoothoperator.Config{})
	require.NoError(t, err)

	// Act
	status, err := op.Status("my-name")

	// Assert
	require.NoError(t, err)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestWorker_WhenHandleReturnsNone_ShouldSleepForIdleDuration(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	h := internal.IdleHandler("idle", 15*time.Millisecond)
	_, err := op.AddHandler("idle", h, smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("idle"))
	time.Sleep(50 * time.Millisecond)

	// Act
	ch, err := op.Stop("idle")
	require.NoError(t, err)
	<-ch

	// Assert
	status, err := op.Status("idle")
	require.NoError(t, err)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestWorkerStop_WhenIdleSleep_ShouldCancelContextAndExitSelect(t *testing.T) {
	t.Parallel()

	// Arrange (long idle so worker is in time.After; Stop() cancels ctx so select gets <-ctx.Done())
	op := smoothoperator.NewOperator(context.Background())
	h := internal.IdleHandler("idle", 5*time.Second)
	_, err := op.AddHandler("idle", h, smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("idle"))
	time.Sleep(50 * time.Millisecond) // let first Handle run and enter idle sleep

	// Act
	ch, err := op.Stop("idle")
	require.NoError(t, err)
	<-ch

	// Assert
	status, err := op.Status("idle")
	require.NoError(t, err)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestWorker_WhenHandleReturnsNoneWithZeroDuration_ShouldNotSleep(t *testing.T) {
	t.Parallel()

	// Arrange (covers handle() path where Status is None but IdleDuration is 0, no select)
	op := smoothoperator.NewOperator(context.Background())
	h := internal.NoneZeroHandler("idle")
	_, err := op.AddHandler("idle", h, smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("idle"))
	time.Sleep(30 * time.Millisecond)

	// Act
	ch, err := op.Stop("idle")
	require.NoError(t, err)
	<-ch

	// Assert
	status, err := op.Status("idle")
	require.NoError(t, err)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestWorker_WhenHandleReturnsFail_ShouldLogErrorAndCanSleep(t *testing.T) {
	t.Parallel()

	// Arrange
	handlerErr := fmt.Errorf("handler failed")
	h := internal.FailHandler("fail", handlerErr, 15*time.Millisecond)
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("fail", h, smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("fail"))
	time.Sleep(50 * time.Millisecond)

	// Act
	ch, err := op.Stop("fail")
	require.NoError(t, err)
	<-ch

	// Assert
	status, err := op.Status("fail")
	require.NoError(t, err)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestWorker_WhenHandlePanics_ShouldRecoverAndContinueUntilStop(t *testing.T) {
	t.Parallel()

	// Arrange: handler that panics every time; no max attempts so worker keeps running
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	op := smoothoperator.NewOperator(ctx)
	_, err := op.AddHandler("panic-worker", internal.PanicHandler("panic-worker"), smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("panic-worker"))

	// Give the worker time to hit the panic and recover at least once
	time.Sleep(100 * time.Millisecond)

	// Act: stop the worker; without panic recovery the goroutine would have died and done might never close
	stopChan, err := op.Stop("panic-worker")
	require.NoError(t, err)

	// Assert: stop channel closes (worker goroutine exited cleanly)
	select {
	case <-stopChan:
		// expected: recovery kept the goroutine alive so it could exit on cancel
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop within timeout; panic recovery may not be working")
	}
	status, err := op.Status("panic-worker")
	require.NoError(t, err)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestWorker_WhenMaxPanicAttemptsReached_ShouldStop(t *testing.T) {
	t.Parallel()

	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("panic-worker", internal.PanicHandler("panic-worker"), smoothoperator.Config{MaxPanicAttempts: 3})
	require.NoError(t, err)
	require.NoError(t, op.Start("panic-worker"))

	// Wait for worker to stop itself after 3 panics (each panic does 1s backoff; 3 panics ~ 3s+)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, _ := op.Status("panic-worker")
		if status == smoothoperator.StatusStopped {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	status, err := op.Status("panic-worker")
	require.NoError(t, err)
	require.Equal(t, smoothoperator.StatusStopped, status, "worker should stop itself after max panic attempts")
}

func TestRemoveHandler_WhenWorkerRunning_ShouldStopAndRemoveWorker(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("worker-1", internal.QuickHandler("worker-1"), smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("worker-1"))
	time.Sleep(20 * time.Millisecond)

	// Act
	err = op.RemoveHandler("worker-1")

	// Assert
	require.NoError(t, err)
	_, statusErr := op.Status("worker-1")
	require.Error(t, statusErr)
	require.Contains(t, statusErr.Error(), "not found")
}

func TestRemoveHandler_WhenWorkerStopped_ShouldRemoveWorker(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("worker-1", internal.QuickHandler("worker-1"), smoothoperator.Config{})
	require.NoError(t, err)

	// Act
	err = op.RemoveHandler("worker-1")

	// Assert
	require.NoError(t, err)
	_, statusErr := op.Status("worker-1")
	require.Error(t, statusErr)
	require.Contains(t, statusErr.Error(), "not found")
}

func TestRemoveHandler_WhenWorkerNotFound_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())

	// Act
	err := op.RemoveHandler("missing")

	// Assert
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestRemoveHandler_WhenWorkerRemoved_ShouldAllowReAddingSameName(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("worker-1", internal.QuickHandler("worker-1"), smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.RemoveHandler("worker-1"))

	// Act: re-register with the same name
	_, err = op.AddHandler("worker-1", internal.QuickHandler("worker-1"), smoothoperator.Config{})

	// Assert
	require.NoError(t, err)
	status, statusErr := op.Status("worker-1")
	require.NoError(t, statusErr)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestStatus_WhenWorkerNotFound_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())

	// Act
	_, err := op.Status("missing")

	// Assert
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// --- Dispatch / SendMessage tests ---

func TestDispatch_WhenWorkerNotFound_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())

	// Act
	delivered, result, err := op.Dispatch(context.Background(), "missing", "hello")

	// Assert
	require.Error(t, err)
	require.Nil(t, delivered)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "not found")
}

func TestSendMessage_WhenWorkerNotFound_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())

	// Act
	delivered, result, err := smoothoperator.SendMessage[string](op, "missing", "hello")

	// Assert
	require.Error(t, err)
	require.Nil(t, delivered)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "not found")
}

func TestDispatch_WhenWorkerRunning_ShouldDeliverMessageToHandler(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	recorder := internal.NewMessageRecorder(5 * time.Second)
	_, err := op.AddHandler("w", recorder, smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond) // let worker start and enter idle

	// Act
	delivered, resultCh, err := op.Dispatch(context.Background(), "w", "hello")
	require.NoError(t, err)

	// Assert: delivered channel closes promptly
	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message was not delivered within timeout")
	}
	// result channel closes after handler finishes (MessageRecorder returns Done() with no Result)
	<-resultCh

	<-op.StopAll()
	msgs := recorder.Messages()
	require.Len(t, msgs, 1)
	require.Equal(t, "hello", msgs[0])
}

func TestSendMessage_WhenWorkerRunning_ShouldDeliverTypedMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	recorder := internal.NewMessageRecorder(5 * time.Second)
	_, err := op.AddHandler("w", recorder, smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond)

	// Act
	delivered, resultCh, err := smoothoperator.SendMessage[string](op, "w", "typed-data")
	require.NoError(t, err)

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message was not delivered within timeout")
	}
	<-resultCh

	<-op.StopAll()

	// Assert: handler received a Message[string] with the correct Data
	msgs := recorder.Messages()
	require.Len(t, msgs, 1)
	m, ok := msgs[0].(smoothoperator.Message[string])
	require.True(t, ok, "expected Message[string], got %T", msgs[0])
	require.Equal(t, "typed-data", m.Data)
}

func TestDispatch_WhenWorkerIdle_ShouldWakeUpAndDeliverImmediately(t *testing.T) {
	t.Parallel()

	// Arrange: worker with a very long idle so it's guaranteed to be sleeping
	op := smoothoperator.NewOperator(context.Background())
	recorder := internal.NewMessageRecorder(10 * time.Second)
	_, err := op.AddHandler("w", recorder, smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond) // let first Handle run and enter idle

	// Act: send a message — should wake the worker from idle
	start := time.Now()
	delivered, resultCh, err := op.Dispatch(context.Background(), "w", "wake-up")
	require.NoError(t, err)

	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message was not delivered within timeout; worker may not have woken from idle")
	}
	<-resultCh
	elapsed := time.Since(start)

	<-op.StopAll()

	// Assert: message was delivered much faster than the 10s idle
	require.Less(t, elapsed, 2*time.Second, "worker should wake from idle immediately on message")
	msgs := recorder.Messages()
	require.Len(t, msgs, 1)
	require.Equal(t, "wake-up", msgs[0])
}

func TestDispatch_WhenMultipleMessages_ShouldDeliverAll(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	recorder := internal.NewMessageRecorder(50 * time.Millisecond)
	_, err := op.AddHandler("w", recorder, smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond)

	// Act: send 3 messages sequentially, waiting for each to be delivered
	for i := 0; i < 3; i++ {
		delivered, resultCh, err := op.Dispatch(context.Background(), "w", fmt.Sprintf("msg-%d", i))
		require.NoError(t, err)
		select {
		case <-delivered:
		case <-time.After(2 * time.Second):
			t.Fatalf("message %d was not delivered within timeout", i)
		}
		<-resultCh
	}

	<-op.StopAll()

	// Assert: all 3 messages were received in order
	msgs := recorder.Messages()
	require.Len(t, msgs, 3)
	require.Equal(t, "msg-0", msgs[0])
	require.Equal(t, "msg-1", msgs[1])
	require.Equal(t, "msg-2", msgs[2])
}

func TestSendMessage_WhenDifferentTypes_ShouldDeliverCorrectTypes(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	recorder := internal.NewMessageRecorder(50 * time.Millisecond)
	_, err := op.AddHandler("w", recorder, smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond)

	// Act: send string then int via the generic SendMessage function
	delivered1, result1, err := smoothoperator.SendMessage[string](op, "w", "hello")
	require.NoError(t, err)
	select {
	case <-delivered1:
	case <-time.After(2 * time.Second):
		t.Fatal("string message not delivered")
	}
	<-result1

	delivered2, result2, err := smoothoperator.SendMessage[int](op, "w", 42)
	require.NoError(t, err)
	select {
	case <-delivered2:
	case <-time.After(2 * time.Second):
		t.Fatal("int message not delivered")
	}
	<-result2

	<-op.StopAll()

	// Assert: both messages arrived with correct types
	msgs := recorder.Messages()
	require.Len(t, msgs, 2)

	strMsg, ok := msgs[0].(smoothoperator.Message[string])
	require.True(t, ok, "expected Message[string], got %T", msgs[0])
	require.Equal(t, "hello", strMsg.Data)

	intMsg, ok := msgs[1].(smoothoperator.Message[int])
	require.True(t, ok, "expected Message[int], got %T", msgs[1])
	require.Equal(t, 42, intMsg.Data)
}

func TestDispatch_WhenHandlerReturnsResult_ShouldReceiveOnResultChannel(t *testing.T) {
	t.Parallel()

	// Arrange: handler that returns DoneWithResult when it receives a message
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.ResultHandler(5*time.Second, "handler-done"), smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond)

	// Act
	delivered, resultCh, err := op.Dispatch(context.Background(), "w", "trigger")
	require.NoError(t, err)

	<-delivered
	result, open := <-resultCh

	// Assert: result channel receives the handler's Result, then closes
	require.True(t, open, "result channel should deliver one value before closing")
	require.Equal(t, "handler-done", result)
	_, open = <-resultCh
	require.False(t, open, "result channel should be closed after sending result")

	<-op.StopAll()
}

func TestHandler_WhenUsingDispatcher_CanSendToOtherWorker(t *testing.T) {
	t.Parallel()

	// Arrange: forwarder sends any message it receives to worker "receiver"
	op := smoothoperator.NewOperator(context.Background())
	receiver := internal.NewMessageRecorder(5 * time.Second)
	_, err := op.AddHandler("receiver", receiver, smoothoperator.Config{})
	require.NoError(t, err)
	_, err = op.AddHandler("forwarder", internal.ForwarderHandler("receiver", 5*time.Second), smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("receiver"))
	require.NoError(t, op.Start("forwarder"))
	time.Sleep(50 * time.Millisecond)

	// Act: send to forwarder; it should forward to receiver via disp.Dispatch
	delivered, resultCh, err := op.Dispatch(context.Background(), "forwarder", "forwarded-msg")
	require.NoError(t, err)
	<-delivered
	<-resultCh

	// Assert: receiver got the message that forwarder sent via the dispatcher
	<-op.StopAll()
	msgs := receiver.Messages()
	require.Len(t, msgs, 1, "receiver should have received one message from forwarder via Dispatcher")
	require.Equal(t, "forwarded-msg", msgs[0])
}

func TestDispatch_WhenContextTimeout_ShouldReturnContextError(t *testing.T) {
	t.Parallel()

	// Worker with buffer 1 and a handler that blocks so the buffer can fill
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.BlockingHandler(2*time.Second), smoothoperator.Config{MessageBufferSize: 1})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond)

	// First message: accepted and worker is now handling it (blocking)
	_, _, err = op.Dispatch(context.Background(), "w", "first")
	require.NoError(t, err)

	// Second message: fills the buffer (cap 1)
	_, _, err = op.Dispatch(context.Background(), "w", "second")
	require.NoError(t, err)

	// Third message: buffer full; Dispatch with short timeout should return deadline exceeded
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _, err = op.Dispatch(ctx, "w", "third")
	require.Error(t, err)
	require.EqualError(t, err, fmt.Sprintf("dispatch timeout: %s", context.DeadlineExceeded.Error()))

	<-op.StopAll()
}

func TestDispatch_WhenMaxDispatchTimeoutSet_ShouldReturnErrorAfterTimeout(t *testing.T) {
	t.Parallel()

	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.BlockingHandler(2*time.Second), smoothoperator.Config{
		MessageBufferSize:  1,
		MaxDispatchTimeout: 50 * time.Millisecond,
	})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond)

	// First message: send and wait until worker has accepted it (so worker is now in handler, blocking)
	delivered1, _, err := op.Dispatch(context.Background(), "w", "first")
	require.NoError(t, err)
	select {
	case <-delivered1:
	case <-time.After(2 * time.Second):
		t.Fatal("first message not delivered")
	}

	// Second message: fills the buffer (worker still in handler)
	_, _, err = op.Dispatch(context.Background(), "w", "second")
	require.NoError(t, err)

	// Third message: buffer full; send is limited by MaxDispatchTimeout (50ms), so should return deadline exceeded
	_, _, err = op.Dispatch(context.Background(), "w", "third")
	require.Error(t, err)
	require.EqualError(t, err, fmt.Sprintf("dispatch timeout: %s", context.DeadlineExceeded.Error()))

	<-op.StopAll()
}

func TestMessageOnly_WhenNoMessageSent_ShouldNeverCallHandler(t *testing.T) {
	t.Parallel()

	op := smoothoperator.NewOperator(context.Background())
	recorder := internal.NewMessageRecorder(5 * time.Second)
	_, err := op.AddHandler("w", recorder, smoothoperator.Config{MessageOnly: true})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))

	// Let the worker run for a bit without any message
	time.Sleep(100 * time.Millisecond)

	<-op.StopAll()
	msgs := recorder.Messages()
	require.Empty(t, msgs, "Handle should never be called when MessageOnly and no message is dispatched")
}

func TestMessageOnly_WhenMessageSent_ShouldCallHandlerWithMessage(t *testing.T) {
	t.Parallel()

	op := smoothoperator.NewOperator(context.Background())
	recorder := internal.NewMessageRecorder(5 * time.Second)
	_, err := op.AddHandler("w", recorder, smoothoperator.Config{MessageOnly: true})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(20 * time.Millisecond)

	delivered, resultCh, err := op.Dispatch(context.Background(), "w", "hello")
	require.NoError(t, err)
	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message not delivered")
	}
	<-resultCh

	<-op.StopAll()
	msgs := recorder.Messages()
	require.Len(t, msgs, 1)
	require.Equal(t, "hello", msgs[0])
}

func TestMessageOnly_WhenMultipleMessagesSent_ShouldDeliverAllInOrder(t *testing.T) {
	t.Parallel()

	op := smoothoperator.NewOperator(context.Background())
	recorder := internal.NewMessageRecorder(50 * time.Millisecond)
	_, err := op.AddHandler("w", recorder, smoothoperator.Config{MessageOnly: true})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(20 * time.Millisecond)

	for i := 0; i < 3; i++ {
		delivered, resultCh, err := op.Dispatch(context.Background(), "w", fmt.Sprintf("msg-%d", i))
		require.NoError(t, err)
		select {
		case <-delivered:
		case <-time.After(2 * time.Second):
			t.Fatalf("message %d not delivered", i)
		}
		<-resultCh
	}

	<-op.StopAll()
	msgs := recorder.Messages()
	require.Len(t, msgs, 3)
	require.Equal(t, "msg-0", msgs[0])
	require.Equal(t, "msg-1", msgs[1])
	require.Equal(t, "msg-2", msgs[2])
}

func TestHandleResult_WhenUsingConstructors_ShouldReturnCorrectStatusAndDuration(t *testing.T) {
	t.Parallel()
	// Arrange
	e := fmt.Errorf("fail")

	// Act
	noneResult := smoothoperator.None(time.Second)
	doneResult := smoothoperator.Done()
	doneWithResult := smoothoperator.DoneWithResult("payload")
	failResult := smoothoperator.Fail(e, 2*time.Second)

	// Assert
	require.Equal(t, smoothoperator.HandleStatusNone, noneResult.Status)
	require.Equal(t, time.Second, noneResult.IdleDuration)
	require.Equal(t, smoothoperator.HandleStatusDone, doneResult.Status)
	require.Equal(t, smoothoperator.HandleStatusDone, doneWithResult.Status)
	require.Equal(t, "payload", doneWithResult.Result)
	require.Equal(t, smoothoperator.HandleStatusFail, failResult.Status)
	require.Equal(t, e, failResult.Err)
	require.Equal(t, 2*time.Second, failResult.IdleDuration)
}

// --- Metrics tests ---

func TestWorker_WhenNotFound_ShouldReturnError(t *testing.T) {
	t.Parallel()
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.Worker("missing")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestWorker_LastMetric_WhenNoEvents_ShouldReturnFalse(t *testing.T) {
	t.Parallel()
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.QuickHandler("w"), smoothoperator.Config{})
	require.NoError(t, err)
	w, err := op.Worker("w")
	require.NoError(t, err)
	_, ok := w.LastMetric()
	require.False(t, ok)
}

func TestWorker_LastMetric_WhenWorkerRuns_ShouldReturnLatestEvent(t *testing.T) {
	t.Parallel()
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.QuickHandler("w"), smoothoperator.Config{})
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	defer func() { <-op.StopAll() }()
	time.Sleep(50 * time.Millisecond)

	w, err := op.Worker("w")
	require.NoError(t, err)
	ev, ok := w.LastMetric()
	require.True(t, ok)
	require.Equal(t, smoothoperator.MetricKindHandle, ev.Kind)
	require.Equal(t, "w", ev.Worker)
	require.Equal(t, smoothoperator.HandleStatusDone, ev.Status)
}

func TestWorker_Metrics_WhenReceiving_ShouldReceiveEventsThenClose(t *testing.T) {
	t.Parallel()
	op := smoothoperator.NewOperator(context.Background())
	recorder := internal.NewMessageRecorder(5 * time.Second)
	_, err := op.AddHandler("w", recorder, smoothoperator.Config{MessageOnly: true})
	require.NoError(t, err)
	w, err := op.Worker("w")
	require.NoError(t, err)
	ch := w.Metrics(10)
	var got []smoothoperator.MetricEvent
	done := make(chan struct{})
	go func() {
		for ev := range ch {
			got = append(got, ev)
		}
		close(done)
	}()
	require.NoError(t, op.Start("w"))
	time.Sleep(20 * time.Millisecond)
	_, _, _ = op.Dispatch(context.Background(), "w", "hello")
	time.Sleep(50 * time.Millisecond)
	<-op.StopAll()
	<-done

	require.NotEmpty(t, got)
	var hasLifecycleStarted, hasLifecycleStopped, hasHandle, hasDispatch bool
	for _, ev := range got {
		switch ev.Kind {
		case smoothoperator.MetricKindLifecycle:
			if ev.LifecycleEvent == "started" {
				hasLifecycleStarted = true
			}
			if ev.LifecycleEvent == "stopped" {
				hasLifecycleStopped = true
			}
		case smoothoperator.MetricKindHandle:
			hasHandle = true
		case smoothoperator.MetricKindDispatch:
			hasDispatch = true
		}
	}
	require.True(t, hasLifecycleStarted, "expected at least one lifecycle started")
	require.True(t, hasLifecycleStopped, "expected lifecycle stopped after StopAll")
	require.True(t, hasHandle, "expected at least one handle event")
	require.True(t, hasDispatch, "expected at least one dispatch event")
}

func TestWorker_Dispatch_ShouldRecordDispatchMetric(t *testing.T) {
	t.Parallel()
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.QuickHandler("w"), smoothoperator.Config{})
	require.NoError(t, err)
	w, _ := op.Worker("w")
	ch := w.Metrics(10)
	var got []smoothoperator.MetricEvent
	done := make(chan struct{})
	go func() {
		for ev := range ch {
			got = append(got, ev)
		}
		close(done)
	}()
	require.NoError(t, op.Start("w"))
	time.Sleep(20 * time.Millisecond)
	_, _, err = op.Dispatch(context.Background(), "w", "ping")
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	<-op.StopAll()
	<-done

	var hasDispatchOk bool
	for _, ev := range got {
		if ev.Kind == smoothoperator.MetricKindDispatch && ev.DispatchOk {
			hasDispatchOk = true
			break
		}
	}
	require.True(t, hasDispatchOk, "expected at least one successful dispatch event in stream")
}
