package workermanager

import (
	"context"
	"testing"
)

type noopHandler struct{}

func (noopHandler) Handle(context.Context) {}

// TestStartAll_WhenStartReturnsError_PropagatesError triggers the error path
// in StartAll by constructing a manager state where the iterated worker's name
// is not in the map (so Start returns "not found").
func TestStartAll_WhenStartReturnsError_PropagatesError(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	wm := NewWorkerManager(ctx).(*workerManager)
	worker := NewWorker("actual-name", noopHandler{})
	wm.workers["key-mismatch"] = worker // key differs from worker.Name()
	err := wm.StartAll()
	if err == nil {
		t.Fatal("expected error from StartAll")
	}
	if err.Error() != "worker actual-name not found" {
		t.Errorf("unexpected error: %v", err)
	}
}
