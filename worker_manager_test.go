package workermanager_test

import (
	"context"
	"fmt"
	"testing"
	"time"
	"workermanager"
	"workermanager/internal"

	"github.com/stretchr/testify/suite"
)

type WorkerManagerTestSuite struct {
	suite.Suite
}

func TestWorkerManager(t *testing.T) {
	suite.Run(t, new(WorkerManagerTestSuite))
}

func (s *WorkerManagerTestSuite) TestStopAll_ShouldWaitAllWorkersToStop() {
	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())
	var workers []*workermanager.Worker
	for _, name := range []string{"worker-1", "worker-2", "worker-3", "worker-4", "worker-5"} {
		worker, err := wm.AddHandler(name, internal.QuickHandler(name))
		s.Require().NoError(err)
		workers = append(workers, worker)
		wm.Start(name)
	}
	time.Sleep(20 * time.Millisecond)

	// Act
	<-wm.StopAll()

	// Assert
	for _, worker := range workers {
		s.Equal(workermanager.StatusStopped, worker.Status())
	}
}

func (s *WorkerManagerTestSuite) TestWorkerManager_WhenStop_ShouldAwaitWorkerStop() {
	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())
	worker, err := wm.AddHandler("worker-1", internal.QuickHandler("worker-1"))
	s.Require().NoError(err)
	wm.Start("worker-1")
	time.Sleep(20 * time.Millisecond)

	// Act
	stopChan, err := wm.Stop("worker-1")
	s.Require().NoError(err)
	<-stopChan

	// Assert
	s.Equal(workermanager.StatusStopped, worker.Status())
}

func (s *WorkerManagerTestSuite) TestAddHandler_WhenDuplicateName_ReturnsError() {
	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())

	// Act
	_, err := wm.AddHandler("a", internal.QuickHandler("a"))
	s.Require().NoError(err)
	_, err = wm.AddHandler("a", internal.QuickHandler("a"))
	s.Require().Error(err)

	// Assert
	s.Contains(err.Error(), "already exists")
}

func (s *WorkerManagerTestSuite) TestStart_WhenWorkerNotFound_ReturnsError() {
	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())

	// Act
	err := wm.Start("missing")
	s.Require().Error(err)

	// Assert
	s.Contains(err.Error(), "not found")
}

func (s *WorkerManagerTestSuite) TestStartAll_StartsAllWorkers() {
	wm := workermanager.NewWorkerManager(context.Background())
	_, _ = wm.AddHandler("w1", internal.QuickHandler("w1"))
	_, _ = wm.AddHandler("w2", internal.QuickHandler("w2"))
	s.Require().NoError(wm.StartAll())
	time.Sleep(20 * time.Millisecond)
	<-wm.StopAll()
}

func (s *WorkerManagerTestSuite) TestStop_WhenWorkerNotFound_ReturnsError() {
	// Arrange
	wm := workermanager.NewWorkerManager(context.Background())

	// Act
	ch, err := wm.Stop("missing")

	// Assert
	s.Require().Error(err)
	s.Nil(ch)
	s.Contains(err.Error(), "not found")
}

func (s *WorkerManagerTestSuite) TestWorker_Start_WhenAlreadyRunning_IsNoOp() {
	ctx := context.Background()
	worker := workermanager.NewWorker("w", internal.QuickHandler("w"))
	s.Require().NoError(worker.Start(ctx))
	s.Require().NoError(worker.Start(ctx)) // second call is no-op
	<-worker.Stop(ctx)
	s.Equal(workermanager.StatusStopped, worker.Status())
}

func (s *WorkerManagerTestSuite) TestWorker_Stop_WhenNotStarted_ReturnsClosedChannelImmediately() {
	worker := workermanager.NewWorker("w", internal.QuickHandler("w"))
	ch := worker.Stop(context.Background())
	s.NotNil(ch)
	_, open := <-ch
	s.False(open, "channel should be closed immediately when worker was never started")
}

func (s *WorkerManagerTestSuite) TestWorker_NameAndStatus() {
	worker := workermanager.NewWorker("my-name", internal.QuickHandler("x"))
	s.Equal("my-name", worker.Name())
	s.Equal(workermanager.StatusStopped, worker.Status())
}

func (s *WorkerManagerTestSuite) TestWorker_WhenHandleReturnsNone_SleepsForIdleDuration() {
	h := internal.IdleHandler("idle", 15*time.Millisecond)
	worker := workermanager.NewWorker("idle", h)
	ctx := context.Background()
	s.Require().NoError(worker.Start(ctx))
	time.Sleep(50 * time.Millisecond)
	ch := worker.Stop(ctx)
	<-ch
	s.Equal(workermanager.StatusStopped, worker.Status())
}

func (s *WorkerManagerTestSuite) TestWorker_WhenIdleSleep_StopCancelsContextAndExitsSelect() {
	// Use long idle so worker is in time.After; Stop() cancels ctx so select gets <-ctx.Done()
	h := internal.IdleHandler("idle", 5*time.Second)
	worker := workermanager.NewWorker("idle", h)
	ctx := context.Background()
	s.Require().NoError(worker.Start(ctx))
	time.Sleep(50 * time.Millisecond) // let first Handle run and enter idle sleep
	ch := worker.Stop(ctx)
	<-ch
	s.Equal(workermanager.StatusStopped, worker.Status())
}

func (s *WorkerManagerTestSuite) TestWorker_WhenHandleReturnsNoneWithZeroDuration_DoesNotSleep() {
	// Covers handle() path where Status is None but IdleDuration is 0 (no select).
	h := internal.NoneZeroHandler("idle")
	worker := workermanager.NewWorker("idle", h)
	ctx := context.Background()
	s.Require().NoError(worker.Start(ctx))
	time.Sleep(30 * time.Millisecond)
	ch := worker.Stop(ctx)
	<-ch
	s.Equal(workermanager.StatusStopped, worker.Status())
}

func (s *WorkerManagerTestSuite) TestWorker_WhenHandleReturnsFail_LogsErrorAndCanSleep() {
	err := fmt.Errorf("handler failed")
	h := internal.FailHandler("fail", err, 15*time.Millisecond)
	worker := workermanager.NewWorker("fail", h)
	ctx := context.Background()
	s.Require().NoError(worker.Start(ctx))
	time.Sleep(50 * time.Millisecond)
	ch := worker.Stop(ctx)
	<-ch
	s.Equal(workermanager.StatusStopped, worker.Status())
}

func (s *WorkerManagerTestSuite) TestHandleResult_NoneDoneFail() {
	s.Equal(workermanager.HandleStatusNone, workermanager.None(time.Second).Status)
	s.Equal(time.Second, workermanager.None(time.Second).IdleDuration)

	s.Equal(workermanager.HandleStatusDone, workermanager.Done().Status)

	e := fmt.Errorf("fail")
	r := workermanager.Fail(e, 2*time.Second)
	s.Equal(workermanager.HandleStatusFail, r.Status)
	s.Equal(e, r.Err)
	s.Equal(2*time.Second, r.IdleDuration)
}
