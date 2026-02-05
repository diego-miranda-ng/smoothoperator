package internal

import (
	"context"
	"fmt"
	"time"
	"workermanager"
)

// handlerMock is a Handler implementation for testing.
type handlerMock struct {
	name string
}

// NewHandler returns a Handler that logs and simulates work each Handle call.
func NewHandler(name string) workermanager.Handler {
	return &handlerMock{name: name}
}

func (h *handlerMock) Handle(ctx context.Context) {
	fmt.Println("worker", h.name, " is working")
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
		// one unit of work simulated
	}
}

// QuickHandler returns a Handler that yields quickly (for tests). Handle exits
// after a short delay or on ctx.Done(), so Stop() completes fast.
func QuickHandler(name string) workermanager.Handler {
	return &quickHandler{name: name}
}

type quickHandler struct {
	name string
}

func (h *quickHandler) Handle(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(10 * time.Millisecond):
	}
}
