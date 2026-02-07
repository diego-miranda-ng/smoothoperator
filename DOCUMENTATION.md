# smoothoperator – Package Documentation

**Module:** `github.com/diego-miranda-ng/smoothoperator`  
**Package:** `workermanager`

This document describes all exported types and methods of the `workermanager` package.

---

## Table of contents

1. [Overview](#overview)
2. [Operator](#operator)
3. [Worker](#worker)
4. [Handler and HandleResult](#handler-and-handleresult)
5. [Constants and types](#constants-and-types)
6. [Usage examples](#usage-examples)

---

## Overview

The package provides a worker-pool abstraction:

- **Operator**: Registers named handlers, starts and stops workers individually or all at once.
- **Worker**: A single runnable unit that wraps a `Handler` and runs it in a loop until stopped.
- **Handler**: Interface with a single method `Handle(ctx) HandleResult` (None / Done / Fail).
- **HandleResult**: Constructors `None`, `Done`, and `Fail` describe the outcome of each `Handle` call and control idle/retry behavior.

Workers run in their own goroutines. When `Handle` returns `None` or `Fail` with a positive `IdleDuration`, the worker sleeps for that duration before the next call. `Stop` cancels the worker’s context and returns a channel that closes when the goroutine has exited.

---

## Operator

The **Operator** manages a set of named workers. You register handlers, then start/stop workers by name or all together. All methods are safe for concurrent use.

### Type

```go
type Operator interface {
    AddHandler(name string, handler Handler) (*Worker, error)
    Start(name string) error
    StartAll() error
    Stop(name string) (chan struct{}, error)
    StopAll() chan struct{}
}
```

### Constructor

#### `NewOperator(ctx context.Context) Operator`

Creates an Operator that uses `ctx` for worker lifecycle. Workers started via this operator run until `ctx` is cancelled or `Stop` / `StopAll` is called.

- **Parameters:** `ctx` – context used for all workers started by this operator.
- **Returns:** An `Operator` implementation (concrete type is unexported).

---

### Methods

#### `AddHandler(name string, handler Handler) (*Worker, error)`

Registers a handler under the given name and returns the corresponding `Worker`. The worker is not started; call `Start` or `StartAll` to run it.

- **Parameters:**
  - `name` – unique identifier for this worker.
  - `handler` – implementation of `Handler` to run.
- **Returns:**
  - `*Worker` – the worker that wraps `handler`.
  - `error` – non-nil if `name` is already registered (e.g. `"worker <name> already exists"`).

---

#### `Start(name string) error`

Starts the worker with the given name. If the worker is already running, this is a no-op and returns `nil`.

- **Parameters:** `name` – the name used when the worker was registered.
- **Returns:** `error` – non-nil if no worker is registered under `name` (e.g. `"worker <name> not found"`).

---

#### `StartAll() error`

Starts every worker currently registered with the operator. If any `Start` fails, it returns that error and does not start remaining workers.

- **Returns:** `error` – first error from `Start`, or `nil` if all started successfully.

---

#### `Stop(name string) (chan struct{}, error)`

Stops the worker with the given name and returns a channel that closes when that worker has fully stopped.

- **Parameters:** `name` – the name of the worker to stop.
- **Returns:**
  - `chan struct{}` – closes when the worker’s goroutine has exited; `nil` if `name` is not found.
  - `error` – non-nil if no worker is registered under `name` (e.g. `"worker <name> not found"`).

Typical usage: `<-op.Stop("worker-name")` to block until the worker has stopped.

---

#### `StopAll() chan struct{}`

Stops all registered workers and returns a channel that closes when every worker has stopped. The operator waits for each worker in turn; the returned channel is closed only after all are stopped.

- **Returns:** `chan struct{}` – always non-nil; closes when all workers have stopped.

Typical usage: `<-op.StopAll()` to block until every worker has stopped.

---

## Worker

A **Worker** wraps a `Handler` and runs it in a loop in a single goroutine until stopped. It exposes name, status, and lifecycle methods.

### Type

```go
type Worker struct {
    // name, handler, status, mu, cancel, done (unexported)
}
```

Use **NewWorker** to create a `*Worker`; the struct fields are not intended for direct access.

### Constructor

#### `NewWorker(name string, handler Handler) *Worker`

Creates a Worker that will run `handler` under the given `name`. The worker starts in `StatusStopped`; call `Start` to run it.

- **Parameters:**
  - `name` – identifier for this worker (e.g. for logging or Operator lookup).
  - `handler` – the `Handler` to run in a loop.
- **Returns:** `*Worker` – always non-nil.

---

### Methods

#### `Name() string`

Returns the name assigned to this worker when it was created.

- **Returns:** the worker’s name string.

---

#### `Status() Status`

Returns the current worker status: `StatusRunning` or `StatusStopped`. Safe to call from any goroutine.

- **Returns:** `Status` – current lifecycle state.

---

#### `Start(ctx context.Context) error`

Starts the worker loop in a new goroutine. The loop runs until `ctx` is cancelled (e.g. by calling `Stop`). Idempotent: if the worker is already running, `Start` returns `nil` without starting a second loop.

- **Parameters:** `ctx` – context used for the worker loop; cancellation stops the loop.
- **Returns:** `error` – always `nil` in the current implementation (idempotent when already running).

---

#### `Stop(ctx context.Context) chan struct{}`

Cancels the worker’s context and returns a channel that closes when the worker goroutine has fully exited. If the worker was never started, returns an already-closed channel immediately. The `ctx` parameter is accepted for API consistency but cancellation is driven by the context passed to `Start`.

- **Parameters:** `ctx` – not used for cancellation; can be `context.Background()`.
- **Returns:** `chan struct{}` – closes when the worker has stopped; never nil.

Typical usage: `<-worker.Stop(ctx)` to block until the worker has stopped.

---

## Handler and HandleResult

### Handler interface

```go
type Handler interface {
    Handle(ctx context.Context) HandleResult
}
```

**Handler** is the interface for the business logic run by a Worker. Implement `Handle` to perform one unit of work; the worker calls it repeatedly until stopped. Return `None`, `Done`, or `Fail` to control sleep and retry behavior.

#### `Handle(ctx context.Context) HandleResult`

Performs one unit of work. Called repeatedly by the worker until the worker is stopped. The worker respects `ctx` cancellation (e.g. when `Stop` is called).

- **Parameters:** `ctx` – request/worker context; check `ctx.Done()` for cancellation.
- **Returns:** `HandleResult` – use `None`, `Done`, or `Fail` to build the result.

---

### HandleResult and HandleStatus

```go
type HandleStatus string

const (
    HandleStatusNone HandleStatus = "none"
    HandleStatusDone HandleStatus = "done"
    HandleStatusFail HandleStatus = "fail"
)

type HandleResult struct {
    Status       HandleStatus
    IdleDuration time.Duration
    Err          error
}
```

- **HandleStatusNone** – No work available; worker sleeps `IdleDuration` before the next `Handle`.
- **HandleStatusDone** – Work was processed; worker continues immediately (no sleep).
- **HandleStatusFail** – Handler failed; `Err` can be logged; worker may sleep `IdleDuration` before retry.

---

### HandleResult constructors

#### `None(idle time.Duration) HandleResult`

Returns a result for “no work available”. The worker sleeps for `idle` before the next `Handle` call. Use `0` to poll without sleeping.

- **Parameters:** `idle` – duration to sleep before the next `Handle`.
- **Returns:** `HandleResult` with `Status: HandleStatusNone`, `IdleDuration: idle`.

---

#### `Done() HandleResult`

Returns a result for “work processed successfully”. The worker does not sleep and proceeds to the next `Handle` immediately.

- **Returns:** `HandleResult` with `Status: HandleStatusDone`.

---

#### `Fail(err error, idle time.Duration) HandleResult`

Returns a result for “handler failed”. `err` is stored for logging; `idle` is the optional backoff before the next attempt. Use `0` for `idle` to retry immediately.

- **Parameters:**
  - `err` – error to attach (e.g. for logging).
  - `idle` – backoff duration before next `Handle`; may be 0.
- **Returns:** `HandleResult` with `Status: HandleStatusFail`, `Err: err`, `IdleDuration: idle`.

---

## Constants and types

### Status

```go
type Status string

const (
    StatusRunning Status = "running"
    StatusStopped Status = "stopped"
)
```

- **StatusRunning** – Worker goroutine is active and calling `Handle`.
- **StatusStopped** – Worker is not running (never started or already stopped).

---

## Usage examples

### Basic: one worker

```go
ctx := context.Background()
op := workermanager.NewOperator(ctx)

worker, err := op.AddHandler("my-worker", myHandler)
if err != nil {
    log.Fatal(err)
}
if err := op.Start("my-worker"); err != nil {
    log.Fatal(err)
}

// ... later ...
stopChan, err := op.Stop("my-worker")
if err != nil {
    log.Fatal(err)
}
<-stopChan
```

### Start all, stop all

```go
op := workermanager.NewOperator(ctx)
op.AddHandler("a", handlerA)
op.AddHandler("b", handlerB)
op.StartAll()
// ...
<-op.StopAll()
```

### Handler returning None / Done / Fail

```go
func (h *MyHandler) Handle(ctx context.Context) workermanager.HandleResult {
    job, err := h.queue.Poll(ctx)
    if err != nil {
        return workermanager.Fail(err, 5*time.Second)
    }
    if job == nil {
        return workermanager.None(1*time.Second)
    }
    h.process(job)
    return workermanager.Done()
}
```

### Direct Worker use (without Operator)

```go
worker := workermanager.NewWorker("standalone", myHandler)
worker.Start(ctx)
// ...
<-worker.Stop(context.Background())
```

---

*Generated for package `github.com/diego-miranda-ng/smoothoperator`.*
