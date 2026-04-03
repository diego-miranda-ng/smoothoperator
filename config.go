package smoothoperator

import "time"

const (
	// defaultPanicBackoff is the duration the worker sleeps after recovering a panic
	// before calling Handle again.
	defaultPanicBackoff = time.Second
	// defaultMessageBufferSize is the capacity of the worker's message channel when not set.
	defaultMessageBufferSize = 1
)

// config holds optional settings for a worker. It is configured via HandlerOption
// when registering a handler with AddHandler.
type config struct {
	maxPanicAttempts   int
	panicBackoff       time.Duration
	messageBufferSize  int
	maxDispatchTimeout time.Duration
	messageOnly        bool
	lockOSThread       bool
}

// HandlerOption configures a worker at registration time. Use WithMaxPanicAttempts,
// WithPanicBackoff, WithMessageBufferSize, WithMaxDispatchTimeout, WithMessageOnly,
// and WithLockOSThread to set optional behavior; zero value applies defaults.
type HandlerOption func(*config)

// WithMaxPanicAttempts sets the maximum number of panic recoveries before the worker
// stops itself. Use 0 for no limit (default).
func WithMaxPanicAttempts(n int) HandlerOption {
	return func(c *config) { c.maxPanicAttempts = n }
}

// WithPanicBackoff sets the duration the worker sleeps after recovering a panic
// before calling Handle again. Use 0 for the default (1 second).
func WithPanicBackoff(d time.Duration) HandlerOption {
	return func(c *config) { c.panicBackoff = d }
}

// WithMessageBufferSize sets the capacity of the worker's incoming message channel.
// Use 0 or 1 for buffer size 1 (default). Larger values allow more messages to queue.
func WithMessageBufferSize(n int) HandlerOption {
	return func(c *config) { c.messageBufferSize = n }
}

// WithMaxDispatchTimeout sets an optional maximum time to wait when sending a message
// to this worker. Use 0 for no timeout (default).
func WithMaxDispatchTimeout(d time.Duration) HandlerOption {
	return func(c *config) { c.maxDispatchTimeout = d }
}

// WithMessageOnly, when true, makes the worker run only when a message is received.
// When false (default), the worker runs in a loop and Handle is called even with no message.
func WithMessageOnly(b bool) HandlerOption {
	return func(c *config) { c.messageOnly = b }
}

// WithLockOSThread, when true, pins the worker's goroutine to a dedicated OS
// thread via runtime.LockOSThread for the lifetime of the run loop. When false
// (default), the goroutine is scheduled normally by the Go runtime.
func WithLockOSThread(b bool) HandlerOption {
	return func(c *config) { c.lockOSThread = b }
}

// applyHandlerOptions applies the given options on top of base and returns the
// resulting config with defaults applied so that messageBufferSize and
// panicBackoff are never zero when the default behavior is desired. When called
// from AddHandler, pass a zero-value config so defaults kick in. When called
// from UpdateHandlerOptions, pass the worker's current config so only the
// supplied options are overridden.
func applyHandlerOptions(base config, opts ...HandlerOption) config {
	for _, opt := range opts {
		opt(&base)
	}
	if base.messageBufferSize <= 0 {
		base.messageBufferSize = defaultMessageBufferSize
	}
	if base.panicBackoff <= 0 {
		base.panicBackoff = defaultPanicBackoff
	}
	return base
}
