package workermanager_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	workermanager "github.com/diego-miranda-ng/smoothoperator"
	"github.com/diego-miranda-ng/smoothoperator/internal"

	"github.com/stretchr/testify/require"
)

func TestStopAll_WhenMultipleWorkersRunning_ShouldWaitAllWorkersToStop(t *testing.T) {
	t.Parallel()

	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())
	var workers []*workermanager.Worker
	for _, name := range []string{"worker-1", "worker-2", "worker-3", "worker-4", "worker-5"} {
		worker, err := wm.AddHandler(name, internal.QuickHandler(name))
		require.NoError(t, err)
		workers = append(workers, worker)
		wm.Start(name)
	}
	time.Sleep(20 * time.Millisecond)

	// Act
	<-wm.StopAll()

	// Assert
	for _, worker := range workers {
		require.Equal(t, workermanager.StatusStopped, worker.Status())
	}
}

func TestStop_WhenWorkerRunning_ShouldAwaitWorkerStop(t *testing.T) {
	t.Parallel()

	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())
	worker, err := wm.AddHandler("worker-1", internal.QuickHandler("worker-1"))
	require.NoError(t, err)
	wm.Start("worker-1")
	time.Sleep(20 * time.Millisecond)

	// Act
	stopChan, err := wm.Stop("worker-1")
	require.NoError(t, err)
	<-stopChan

	// Assert
	require.Equal(t, workermanager.StatusStopped, worker.Status())
}

func TestAddHandler_WhenDuplicateName_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())

	// Act
	_, err := wm.AddHandler("a", internal.QuickHandler("a"))
	require.NoError(t, err)
	_, err = wm.AddHandler("a", internal.QuickHandler("a"))
	require.Error(t, err)

	// Assert
	require.Contains(t, err.Error(), "already exists")
}

func TestStart_WhenWorkerNotFound_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())

	// Act
	err := wm.Start("missing")
	require.Error(t, err)

	// Assert
	require.Contains(t, err.Error(), "not found")
}

func TestStartAll_WhenWorkersAdded_ShouldStartAllWorkers(t *testing.T) {
	t.Parallel()

	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())
	_, _ = wm.AddHandler("w1", internal.QuickHandler("w1"))
	_, _ = wm.AddHandler("w2", internal.QuickHandler("w2"))

	// Act
	require.NoError(t, wm.StartAll())
	time.Sleep(20 * time.Millisecond)
	<-wm.StopAll()

	// Assert
	// (implicit: StopAll completes without hanging)
}

func TestStop_WhenWorkerNotFound_ShouldReturnError(t *testing.T) {
	t.Parallel()

	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())

	// Act
	ch, err := wm.Stop("missing")

	// Assert
	require.Error(t, err)
	require.Nil(t, ch)
	require.Contains(t, err.Error(), "not found")
}

func TestWorkerStart_WhenAlreadyRunning_ShouldBeNoOp(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	worker := workermanager.NewWorker("w", internal.QuickHandler("w"))
	require.NoError(t, worker.Start(ctx))

	// Act
	require.NoError(t, worker.Start(ctx)) // second call is no-op
	<-worker.Stop(ctx)

	// Assert
	require.Equal(t, workermanager.StatusStopped, worker.Status())
}

func TestWorkerStop_WhenNotStarted_ShouldReturnClosedChannelImmediately(t *testing.T) {
	t.Parallel()

	// Arrange
	worker := workermanager.NewWorker("w", internal.QuickHandler("w"))

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
	worker := workermanager.NewWorker("my-name", internal.QuickHandler("x"))

	// Act
	name := worker.Name()
	status := worker.Status()

	// Assert
	require.Equal(t, "my-name", name)
	require.Equal(t, workermanager.StatusStopped, status)
}

func TestWorker_WhenHandleReturnsNone_ShouldSleepForIdleDuration(t *testing.T) {
	t.Parallel()

	// Arrange
	h := internal.IdleHandler("idle", 15*time.Millisecond)
	worker := workermanager.NewWorker("idle", h)
	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))
	time.Sleep(50 * time.Millisecond)

	// Act
	ch := worker.Stop(ctx)
	<-ch

	// Assert
	require.Equal(t, workermanager.StatusStopped, worker.Status())
}

func TestWorkerStop_WhenIdleSleep_ShouldCancelContextAndExitSelect(t *testing.T) {
	t.Parallel()

	// Arrange (long idle so worker is in time.After; Stop() cancels ctx so select gets <-ctx.Done())
	h := internal.IdleHandler("idle", 5*time.Second)
	worker := workermanager.NewWorker("idle", h)
	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))
	time.Sleep(50 * time.Millisecond) // let first Handle run and enter idle sleep

	// Act
	ch := worker.Stop(ctx)
	<-ch

	// Assert
	require.Equal(t, workermanager.StatusStopped, worker.Status())
}

func TestWorker_WhenHandleReturnsNoneWithZeroDuration_ShouldNotSleep(t *testing.T) {
	t.Parallel()

	// Arrange (covers handle() path where Status is None but IdleDuration is 0, no select)
	h := internal.NoneZeroHandler("idle")
	worker := workermanager.NewWorker("idle", h)
	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))
	time.Sleep(30 * time.Millisecond)

	// Act
	ch := worker.Stop(ctx)
	<-ch

	// Assert
	require.Equal(t, workermanager.StatusStopped, worker.Status())
}

func TestWorker_WhenHandleReturnsFail_ShouldLogErrorAndCanSleep(t *testing.T) {
	t.Parallel()

	// Arrange
	err := fmt.Errorf("handler failed")
	h := internal.FailHandler("fail", err, 15*time.Millisecond)
	worker := workermanager.NewWorker("fail", h)
	ctx := context.Background()
	require.NoError(t, worker.Start(ctx))
	time.Sleep(50 * time.Millisecond)

	// Act
	ch := worker.Stop(ctx)
	<-ch

	// Assert
	require.Equal(t, workermanager.StatusStopped, worker.Status())
}

func TestHandleResult_WhenUsingConstructors_ShouldReturnCorrectStatusAndDuration(t *testing.T) {
	t.Parallel()
	// Arrange
	e := fmt.Errorf("fail")

	// Act
	noneResult := workermanager.None(time.Second)
	doneResult := workermanager.Done()
	failResult := workermanager.Fail(e, 2*time.Second)

	// Assert
	require.Equal(t, workermanager.HandleStatusNone, noneResult.Status)
	require.Equal(t, time.Second, noneResult.IdleDuration)
	require.Equal(t, workermanager.HandleStatusDone, doneResult.Status)
	require.Equal(t, workermanager.HandleStatusFail, failResult.Status)
	require.Equal(t, e, failResult.Err)
	require.Equal(t, 2*time.Second, failResult.IdleDuration)
}
