// Package smoothoperator provides a worker pool abstraction for running named
// handlers in goroutines with coordinated start/stop and configurable idle behavior.
//
// # Overview
//
// The package exposes:
//   - Operator: register handlers by name, start/stop workers by name, query status by name.
//   - Handler: interface with Handle(ctx) returning HandleResult (None/Done/Fail).
//   - HandleResult constructors: None, Done, Fail for building handler responses.
//   - Config: optional settings per worker (e.g. MaxPanicAttempts, PanicBackoff).
//   - Status: worker state (StatusRunning, StatusStopped).
//
// Workers are not exposed; they are managed entirely by the Operator.
//
// # Usage
//
// Create an operator with a context (used for all workers), add handlers, then start/stop:
//
//	ctx := context.Background()
//	op := smoothoperator.NewOperator(ctx)
//	_, err := op.AddHandler("my-worker", myHandler, smoothoperator.Config{})
//	if err != nil { ... }
//	op.Start("my-worker")
//	// ... later ...
//	<-op.Stop("my-worker")  // wait for stop
//	// or stop all: <-op.StopAll()
//	status, _ := op.Status("my-worker")
//
// Workers run in a loop: Handle is called; if the result is None or Fail with IdleDuration,
// the worker sleeps for that duration before the next Handle call. If Handle panics, the
// worker recovers, logs the panic, sleeps briefly, and continues. Stop cancels the context
// and returns a channel that closes when the worker has fully stopped.
package smoothoperator
