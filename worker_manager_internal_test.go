package workermanager

import (
	"context"
	"testing"
)

type noopHandler struct{}

func (noopHandler) Handle(context.Context) HandleResult { return Done() }

// TestStartAll_WhenStartReturnsError_PropagatesError covers the error-return path in StartAll.
func TestStartAll_WhenStartReturnsError_PropagatesError(t *testing.T) {
	ctx := context.Background()
	wm := NewWorkerManager(ctx).(*workerManager)
	worker := NewWorker("actual-name", noopHandler{})
	wm.workers["key-mismatch"] = worker
	err := wm.StartAll()
	if err == nil {
		t.Fatal("expected error from StartAll")
	}
	if err.Error() != "worker actual-name not found" {
		t.Errorf("unexpected error: %v", err)
	}
}
