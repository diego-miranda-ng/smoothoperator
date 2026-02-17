package smoothoperator_test

import (
	"context"
	"log/slog"
	"sync"
)

// captureHandler is a slog.Handler that records log messages for testing.
type captureHandler struct {
	mu      sync.Mutex
	records []string
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	h.records = append(h.records, r.Message)
	h.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (h *captureHandler) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.records...)
}
