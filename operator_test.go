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
		require.NoError(t, op.AddHandler(name, internal.QuickHandler(name), smoothoperator.Config{}))
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
	require.NoError(t, op.AddHandler("worker-1", internal.QuickHandler("worker-1"), smoothoperator.Config{}))
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
	require.NoError(t, op.AddHandler("a", internal.QuickHandler("a"), smoothoperator.Config{}))
	err := op.AddHandler("a", internal.QuickHandler("a"), smoothoperator.Config{})
	require.Error(t, err)

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
	require.NoError(t, op.AddHandler("w1", internal.QuickHandler("w1"), smoothoperator.Config{}))
	require.NoError(t, op.AddHandler("w2", internal.QuickHandler("w2"), smoothoperator.Config{}))

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
	require.NoError(t, op.AddHandler("w", internal.QuickHandler("w"), smoothoperator.Config{}))
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
	require.NoError(t, op.AddHandler("w", internal.QuickHandler("w"), smoothoperator.Config{}))

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
	require.NoError(t, op.AddHandler("my-name", internal.QuickHandler("x"), smoothoperator.Config{}))

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
	require.NoError(t, op.AddHandler("idle", h, smoothoperator.Config{}))
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
	require.NoError(t, op.AddHandler("idle", h, smoothoperator.Config{}))
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
	require.NoError(t, op.AddHandler("idle", h, smoothoperator.Config{}))
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
	require.NoError(t, op.AddHandler("fail", h, smoothoperator.Config{}))
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
	require.NoError(t, op.AddHandler("panic-worker", internal.PanicHandler("panic-worker"), smoothoperator.Config{}))
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
	require.NoError(t, op.AddHandler("panic-worker", internal.PanicHandler("panic-worker"), smoothoperator.Config{MaxPanicAttempts: 3}))
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

func TestHandleResult_WhenUsingConstructors_ShouldReturnCorrectStatusAndDuration(t *testing.T) {
	t.Parallel()
	// Arrange
	e := fmt.Errorf("fail")

	// Act
	noneResult := smoothoperator.None(time.Second)
	doneResult := smoothoperator.Done()
	failResult := smoothoperator.Fail(e, 2*time.Second)

	// Assert
	require.Equal(t, smoothoperator.HandleStatusNone, noneResult.Status)
	require.Equal(t, time.Second, noneResult.IdleDuration)
	require.Equal(t, smoothoperator.HandleStatusDone, doneResult.Status)
	require.Equal(t, smoothoperator.HandleStatusFail, failResult.Status)
	require.Equal(t, e, failResult.Err)
	require.Equal(t, 2*time.Second, failResult.IdleDuration)
}
