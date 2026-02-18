package smoothoperator_test

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// captureEntry is the internal representation of a log record inside the store.
type captureEntry struct {
	message string
	level   slog.Level
	attrs   map[string]string
}

// captureStore is a shared, thread-safe store for log records. Multiple
// captureHandler instances (created by WithAttrs) write to the same store.
type captureStore struct {
	mu      sync.Mutex
	entries []captureEntry
}

func (s *captureStore) append(e captureEntry) {
	s.mu.Lock()
	s.entries = append(s.entries, e)
	s.mu.Unlock()
}

func (s *captureStore) all() []captureEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]captureEntry, len(s.entries))
	copy(out, s.entries)
	return out
}

// captureHandler is a slog.Handler that records log messages and their
// structured attributes for testing. It correctly propagates attributes
// added via WithAttrs so that default logger fields (e.g. from initLogger)
// appear in every captured record.
//
// When returned by records() or findByMessage(), the Message, Level, and
// Attrs fields are populated to represent a single log entry.
type captureHandler struct {
	store    *captureStore
	preAttrs []slog.Attr

	Message string
	Level   slog.Level
	Attrs   map[string]string
}

func newCaptureHandler() *captureHandler {
	return &captureHandler{store: &captureStore{}}
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	attrs := make(map[string]string, len(h.preAttrs)+r.NumAttrs())
	for _, a := range h.preAttrs {
		flattenAttr("", a, attrs)
	}
	r.Attrs(func(a slog.Attr) bool {
		flattenAttr("", a, attrs)
		return true
	})
	h.store.append(captureEntry{
		message: r.Message,
		level:   r.Level,
		attrs:   attrs,
	})
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, len(h.preAttrs), len(h.preAttrs)+len(attrs))
	copy(merged, h.preAttrs)
	merged = append(merged, attrs...)
	return &captureHandler{store: h.store, preAttrs: merged}
}

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

// flattenAttr converts an slog.Attr to one or more string entries in the map.
// Group values are flattened with dot-separated keys.
func flattenAttr(prefix string, a slog.Attr, out map[string]string) {
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	if a.Value.Kind() == slog.KindGroup {
		for _, ga := range a.Value.Group() {
			flattenAttr(key, ga, out)
		}
		return
	}
	out[key] = fmt.Sprintf("%v", a.Value.Any())
}

func entryToHandler(e captureEntry) *captureHandler {
	return &captureHandler{
		Message: e.message,
		Level:   e.level,
		Attrs:   e.attrs,
	}
}

// messages returns just the message strings (backwards compatible with old tests).
func (h *captureHandler) messages() []string {
	entries := h.store.all()
	msgs := make([]string, len(entries))
	for i, e := range entries {
		msgs[i] = e.message
	}
	return msgs
}

// records returns all captured records as captureHandler instances for detailed assertions.
func (h *captureHandler) records() []*captureHandler {
	entries := h.store.all()
	out := make([]*captureHandler, len(entries))
	for i, e := range entries {
		out[i] = entryToHandler(e)
	}
	return out
}

// findByMessage returns all records whose message starts with the given prefix,
// each represented as a captureHandler with Message, Level, and Attrs populated.
func (h *captureHandler) findByMessage(prefix string) []*captureHandler {
	var out []*captureHandler
	for _, e := range h.store.all() {
		if strings.HasPrefix(e.message, prefix) {
			out = append(out, entryToHandler(e))
		}
	}
	return out
}
