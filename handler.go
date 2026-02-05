package workermanager

import (
	"context"
	"time"
)

// HandleStatus is the result of a Handle call. It tells the worker whether work
// was done, nothing was available (idle), or the handler failed.
type HandleStatus string

const (
	HandleStatusNone HandleStatus = "none" // No work; worker sleeps IdleDuration
	HandleStatusDone HandleStatus = "done" // Work processed; continue
	HandleStatusFail HandleStatus = "fail" // Error; Err is set for logging, worker can sleep IdleDuration
)

// HandleResult is returned by Handler.Handle. Status controls whether the
// worker sleeps; when None or Fail, IdleDuration is the time to sleep.
type HandleResult struct {
	Status       HandleStatus
	IdleDuration time.Duration // When None (or Fail with backoff): time to sleep before next Handle
	Err          error         // When Status == Fail: error to log
}

// None returns a result for when there was no work. Worker sleeps for d.
func None(idle time.Duration) HandleResult {
	return HandleResult{Status: HandleStatusNone, IdleDuration: idle}
}

// Done returns a result for when work was processed. Worker does not sleep.
func Done() HandleResult {
	return HandleResult{Status: HandleStatusDone}
}

// Fail returns a result for when the handler failed. Err is for logging; idle is optional backoff before the next attempt.
func Fail(err error, idle time.Duration) HandleResult {
	return HandleResult{Status: HandleStatusFail, IdleDuration: idle, Err: err}
}

// Handler is the minimal interface for business logic. Handlers are added to
// the work manager and wrapped by a Worker which manages state and lifecycle.
// Handle returns a status: None (sleep IdleDuration), Done (continue), or Fail (log Err, optional sleep).
type Handler interface {
	Handle(ctx context.Context) HandleResult
}
