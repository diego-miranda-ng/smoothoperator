package smoothoperator

import "errors"

// Sentinel errors for operator operations. Use errors.Is to check the error type:
//
//	_, err := op.AddHandler("worker", h)
//	if errors.Is(err, ErrWorkerAlreadyExists) { ... }
//
//	_, err := op.Start("missing")
//	if errors.Is(err, ErrWorkerNotFound) { ... }
//
//	_, _, err := op.Dispatch(ctx, "w", msg)
//	if errors.Is(err, ErrDispatchTimeout) { ... }
//	if errors.Is(err, context.DeadlineExceeded) { ... } // underlying context error
var (
	// ErrWorkerAlreadyExists is returned when AddHandler is called with a name
	// that is already registered.
	ErrWorkerAlreadyExists = errors.New("worker already exists")

	// ErrWorkerNotFound is returned when an operation (Start, Stop, RemoveHandler,
	// Status, Worker, Dispatch) is called with a worker name that is not registered.
	ErrWorkerNotFound = errors.New("worker not found")

	// ErrDispatchTimeout is returned when Dispatch cannot send the message before
	// the context is cancelled or the worker's max dispatch timeout is exceeded.
	// Use errors.Unwrap or errors.As to inspect the underlying context error
	// (e.g. context.DeadlineExceeded, context.Canceled).
	ErrDispatchTimeout = errors.New("dispatch timeout")
)
