package smoothoperator

import (
	"context"
	"testing"
)

type noopHandler struct{}

func (noopHandler) Handle(context.Context) HandleResult { return Done() }

// TestStartAll_WhenStartReturnsError_PropagatesError covers the error-return path in StartAll.
func TestStartAll_WhenStartReturnsError_PropagatesError(t *testing.T) {
	ctx := context.Background()
	op := NewOperator(ctx).(*operator)
	worker := NewWorker("actual-name", noopHandler{})
	op.workers["key-mismatch"] = worker
	err := op.StartAll()
	if err == nil {
		t.Fatal("expected error from StartAll")
	}
	if err.Error() != "worker actual-name not found" {
		t.Errorf("unexpected error: %v", err)
	}
}
