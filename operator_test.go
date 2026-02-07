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
	var workers []*smoothoperator.Worker
	for _, name := range []string{"worker-1", "worker-2", "worker-3", "worker-4", "worker-5"} {
		worker, err := op.AddHandler(name, internal.QuickHandler(name), smoothoperator.Config{})
		require.NoError(t, err)
		workers = append(workers, worker)
		op.Start(name)
	}
	time.Sleep(20 * time.Millisecond)

	// Act
	<-op.StopAll()

	// Assert
	for _, worker := range workers {
		require.Equal(t, smoothoperator.StatusStopped, worker.Status())
	}
}

func TestStop_WhenWorkerRunning_ShouldAwaitWorkerStop(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	worker, err := op.AddHandler("worker-1", internal.QuickHandler("worker-1"), smoothoperator.Config{})
	require.NoError(t, err)
	op.Start("worker-1")
	time.Sleep(20 * time.Millisecond)

	// Act
	stopChan, err := op.Stop("worker-1")
	require.NoError(t, err)
	<-stopChan

	// Assert
	require.Equal(t, smoothoperator.StatusStopped, worker.Status())
}

func TestAddHandler_WhenDuplicateName_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	op := smoothoperator.NewOperator(context.Background())

	// Act
	_, err := op.AddHandler("a", internal.QuickHandler("a"), smoothoperator.Config{})
	require.NoError(t, err)
	_, err = op.AddHandler("a", internal.QuickHandler("a"), smoothoperator.Config{})
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
	_, _ = op.AddHandler("w1", internal.QuickHandler("w1"), smoothoperator.Config{})
	_, _ = op.AddHandler("w2", internal.QuickHandler("w2"), smoothoperator.Config{})

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
	ctx := context.Background()
	worker := smoothoperator.NewWorker("w", internal.QuickHandler("w"), smoothoperator.Config{})
	require.NoError(t, worker.Start(ctx))

	// Act
	require.NoError(t, worker.Start(ctx)) // second call is no-op
	<-worker.Stop(ctx)

	// Assert
	require.Equal(t, smoothoperator.StatusStopped, worker.Status())
}

func TestWorkerStop_WhenNotStarted_ShouldReturnClosedChannelImmediately(t *testing.T) {
	t.Parallel()

	// Arrange
	worker := smoothoperator.NewWorker("w", internal.QuickHandler("w"), smoothoperator.Config{})

	// Act
	ch := worker.Stop(context.Background())

	// Assert
	require.NotNil(t, ch)
	_, open := <-ch
	require.False(t, open, "channel should be closed immediately when worker was never started")
}

func TestWorker_WhenCreated_ShouldReturnNameAndStoppedStatus(t *testing.T) {
	t.Parallel()

	// Arrange
	worker := smoothoperator.NewWorker("my-name", internal.QuickHandler("x"), smoothoperator.Config{})

	// Act
	name := worker.Name()
	status := worker.Status()

	// Assert
	require.Equal(t, "my-name", name)
	require.Equal(t, smoothoperator.StatusStopped, status)
}

func TestWorker_WhenHandleReturnsNone_ShouldSleepForIdleDuration(t *testing.T) {
	t.Parallel()

	// Arrange
	h := internal.IdleHandler("idle", 15*time.Millisecond)
	worker := smoothoperator.NewWorker("idle", h, smoothoperator.Config{})
	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))
	time.Sleep(50 * time.Millisecond)

	// Act
	ch := worker.Stop(ctx)
	<-ch

	// Assert
	require.Equal(t, smoothoperator.StatusStopped, worker.Status())
}

func TestWorkerStop_WhenIdleSleep_ShouldCancelContextAndExitSelect(t *testing.T) {
	t.Parallel()

	// Arrange (long idle so worker is in time.After; Stop() cancels ctx so select gets <-ctx.Done())
	h := internal.IdleHandler("idle", 5*time.Second)
	worker := smoothoperator.NewWorker("idle", h, smoothoperator.Config{})
	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))
	time.Sleep(50 * time.Millisecond) // let first Handle run and enter idle sleep

	// Act
	ch := worker.Stop(ctx)
	<-ch

	// Assert
	require.Equal(t, smoothoperator.StatusStopped, worker.Status())
}

func TestWorker_WhenHandleReturnsNoneWithZeroDuration_ShouldNotSleep(t *testing.T) {
	t.Parallel()

	// Arrange (covers handle() path where Status is None but IdleDuration is 0, no select)
	h := internal.NoneZeroHandler("idle")
	worker := smoothoperator.NewWorker("idle", h, smoothoperator.Config{})
	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))
	time.Sleep(30 * time.Millisecond)

	// Act
	ch := worker.Stop(ctx)
	<-ch

	// Assert
	require.Equal(t, smoothoperator.StatusStopped, worker.Status())
}

func TestWorker_WhenHandleReturnsFail_ShouldLogErrorAndCanSleep(t *testing.T) {
	t.Parallel()

	// Arrange
	err := fmt.Errorf("handler failed")
	h := internal.FailHandler("fail", err, 15*time.Millisecond)
	worker := smoothoperator.NewWorker("fail", h, smoothoperator.Config{})
	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))
	time.Sleep(50 * time.Millisecond)

	// Act
	ch := worker.Stop(ctx)
	<-ch

	// Assert
	require.Equal(t, smoothoperator.StatusStopped, worker.Status())
}

func TestWorker_WhenHandlePanics_ShouldRecoverAndContinueUntilStop(t *testing.T) {
	t.Parallel()

	// Arrange: handler that panics every time; no max attempts so worker keeps running
	worker := smoothoperator.NewWorker("panic-worker", internal.PanicHandler("panic-worker"), smoothoperator.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, worker.Start(ctx))

	// Give the worker time to hit the panic and recover at least once
	time.Sleep(100 * time.Millisecond)

	// Act: stop the worker; without panic recovery the goroutine would have died and done might never close
	stopChan := worker.Stop(context.Background())

	// Assert: stop channel closes (worker goroutine exited cleanly)
	select {
	case <-stopChan:
		// expected: recovery kept the goroutine alive so it could exit on cancel
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not stop within timeout; panic recovery may not be working")
	}
	require.Equal(t, smoothoperator.StatusStopped, worker.Status())
}

func TestWorker_WhenMaxPanicAttemptsReached_ShouldStop(t *testing.T) {
	t.Parallel()

	worker := smoothoperator.NewWorker("panic-worker", internal.PanicHandler("panic-worker"), smoothoperator.Config{MaxPanicAttempts: 3})
	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))

	// Wait for worker to stop itself after 3 panics (each panic does 1s backoff; 3 panics ~ 3s+)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if worker.Status() == smoothoperator.StatusStopped {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.Equal(t, smoothoperator.StatusStopped, worker.Status(), "worker should stop itself after max panic attempts")
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
