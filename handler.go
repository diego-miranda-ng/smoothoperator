package smoothoperator

import (
	"context"
	"time"
)

// HandleStatus is the result of a Handle call. It tells the worker whether work
// was done, nothing was available (idle), or the handler failed.
type HandleStatus string

const (
	// HandleStatusNone means no work was available. The worker sleeps for IdleDuration
	// before calling Handle again.
	HandleStatusNone HandleStatus = "none"
	// HandleStatusDone means work was processed successfully. The worker continues
	// immediately to the next Handle call without sleeping.
	HandleStatusDone HandleStatus = "done"
	// HandleStatusFail means the handler encountered an error. Err is set for logging;
	// if IdleDuration > 0, the worker sleeps before retrying.
	HandleStatusFail HandleStatus = "fail"
)

// HandleResult is returned by Handler.Handle. Status controls whether the
// worker sleeps; when None or Fail, IdleDuration is the time to sleep before
// the next Handle call.
type HandleResult struct {
	// Status indicates the outcome of this Handle invocation.
	Status HandleStatus
	// IdleDuration is used when Status is None or Fail. The worker sleeps for this
	// duration before calling Handle again. Ignored when Status is Done.
	IdleDuration time.Duration
	// Err is set when Status is Fail; it can be logged by the worker. Optional.
	Err error
	// Result is optional data returned to the caller of Send. When set, it is sent
	// on the result channel returned by Send after the handler finishes execution.
	Result any
}

// None returns a HandleResult for when there was no work to do. The worker
// sleeps for idle before the next Handle call. Use zero duration to poll
// without sleeping.
func None(idle time.Duration) HandleResult {
	return HandleResult{Status: HandleStatusNone, IdleDuration: idle}
}

// Done returns a HandleResult for when work was processed successfully. The
// worker does not sleep and proceeds to the next Handle call immediately.
func Done() HandleResult {
	return HandleResult{Status: HandleStatusDone}
}

// DoneWithResult returns a HandleResult for when work was processed successfully
// and the handler wants to return data to the caller of Send. The result is sent
// on the result channel returned by Send after the handler finishes.
func DoneWithResult(result any) HandleResult {
	return HandleResult{Status: HandleStatusDone, Result: result}
}

// Fail returns a HandleResult for when the handler failed. err is stored for
// logging; idle is the optional backoff duration before the next Handle attempt.
// Pass 0 for idle to retry immediately.
func Fail(err error, idle time.Duration) HandleResult {
	return HandleResult{Status: HandleStatusFail, IdleDuration: idle, Err: err}
}

// DispatcherAware is an optional interface. If a Handler implements it,
// SetDispatcher is called once with the Operator's Dispatcher when the handler
// is registered via AddHandler. The handler can store it and use it to send
// messages to other workers. Handlers that do not need to dispatch need not
// implement this interface.
type DispatcherAware interface {
	SetDispatcher(disp Dispatcher)
}

// Handler is the interface for business logic run by a Worker. Implement Handle
// to perform one unit of work; return None (with optional idle), Done, or Fail.
// Handlers are registered with an Operator via AddHandler and wrapped in a Worker.
// To send messages to other workers, implement the optional DispatcherAware
// interface; SetDispatcher will be called once at registration.
type Handler interface {
	// Handle performs one unit of work. It is called repeatedly by the worker until
	// the worker is stopped. The msg parameter carries a message sent via SendMessage;
	// it is nil when no message was sent. Return None/Done/Fail to control sleep
	// and retry behavior.
	Handle(ctx context.Context, msg any) HandleResult
}
