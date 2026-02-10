# smoothoperator – Package Documentation

**Module:** `github.com/diego-miranda-ng/smoothoperator`  
**Package:** `smoothoperator`

This document describes all exported types and methods of the `smoothoperator` package.

---

## Table of contents

1. [Overview](#overview)
2. [Operator](#operator)
3. [Workers (internal)](#workers-internal)
4. [Metrics and Worker](#metrics-and-worker)
5. [Handler and HandleResult](#handler-and-handleresult)
6. [Constants and types](#constants-and-types)
7. [Usage examples](#usage-examples)

---

## Overview

The package provides a worker-pool abstraction:

- **Operator**: Registers named handlers, starts and stops workers individually or all at once.
- **Workers**: Created and managed by the Operator by name; not exposed to callers.
- **Handler**: Interface with a single method `Handle(ctx) HandleResult` (None / Done / Fail).
- **HandleResult**: Constructors `None`, `Done`, and `Fail` describe the outcome of each `Handle` call and control idle/retry behavior.

Workers run in their own goroutines. When `Handle` returns `None` or `Fail` with a positive `IdleDuration`, the worker sleeps for that duration before the next call. `Stop` cancels the worker’s context and returns a channel that closes when the goroutine has exited.

---

## Operator

The **Operator** manages a set of named workers. You register handlers, then start/stop workers by name or all together. All methods are safe for concurrent use.

### Type

```go
type Operator interface {
    AddHandler(name string, handler Handler, config Config) (Worker, error)
    Start(name string) error
    StartAll() error
    Stop(name string) (chan struct{}, error)
    StopAll() chan struct{}
    Status(name string) (Status, error)
    Worker(name string) (Worker, error)
}
```

### Config

```go
type Config struct {
    MaxPanicAttempts int           // max panic recoveries before worker stops; 0 = no limit
    PanicBackoff     time.Duration // sleep after recovering a panic; 0 = 1s default
}
```

`Config` is passed to `AddHandler` to configure the worker. Use a zero value `Config{}` for defaults.

### Constructor

#### `NewOperator(ctx context.Context) Operator`

Creates an Operator that uses `ctx` for worker lifecycle. Workers started via this operator run until `ctx` is cancelled or `Stop` / `StopAll` is called.

- **Parameters:** `ctx` – context used for all workers started by this operator.
- **Returns:** An `Operator` implementation (concrete type is unexported).

---

### Methods

#### `AddHandler(name string, handler Handler, config Config) (Worker, error)`

Registers a handler under the given name with the given config. The worker is not started; call `Start` or `StartAll` to run it. Returns the **Worker** interface for metrics access (see [Metrics and Worker](#metrics-and-worker)).

- **Parameters:**
  - `name` – unique identifier for this worker.
  - `handler` – implementation of `Handler` to run.
  - `config` – worker config (e.g. `MaxPanicAttempts`); use `Config{}` for defaults.
- **Returns:**
  - `Worker` – interface for metrics (`Metrics()` and `LastMetric()`); `nil` if an error occurred.
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

#### `Status(name string) (Status, error)`

Returns the current status of the worker with the given name (`StatusRunning` or `StatusStopped`).

- **Parameters:** `name` – the name used when the worker was registered.
- **Returns:**
  - `Status` – current state of the worker.
  - `error` – non-nil if no worker is registered under `name` (e.g. `"worker <name> not found"`).

---

#### `Worker(name string) (Worker, error)`

Returns the **Worker** interface for the given name, used to read metrics (stream or last snapshot). See [Metrics and Worker](#metrics-and-worker).

- **Parameters:** `name` – the name used when the worker was registered.
- **Returns:**
  - `Worker` – interface with `Metrics()` and `LastMetric()`.
  - `error` – non-nil if no worker is registered under `name` (e.g. `"worker <name> not found"`).

---

## Workers (internal)

Workers are not exposed by the package. The **Operator** creates and manages them by name. If a handler’s `Handle` panics, the worker recovers, logs the panic, sleeps for a short backoff, then continues the loop so the goroutine does not exit and `Stop` can complete normally.

### Panic recovery and max attempts

If a handler’s `Handle` method panics, the worker recovers inside the loop. The panic value is converted to an error and logged (via the standard `log` package), the panic count is incremented, then the worker sleeps for a fixed backoff (one second) before calling `Handle` again. Context cancellation is respected during this sleep, so `Stop` still causes the worker to exit promptly.

You can set a maximum number of panic recoveries by passing **Config{MaxPanicAttempts: n}** to `AddHandler`. When the count reaches that limit, the worker logs that the limit was reached, cancels its own context, and exits. Use `0` (the default) for no limit.

Handle errors (when the handler returns `Fail` with a non-nil `Err`) are also logged with the standard `log` package.

---

## Metrics and Worker

You can observe worker execution via metrics. Obtain a **Worker** from the operator with `Operator.Worker(name)`. The Worker interface provides two ways to get metrics:

- **Metrics()** – returns a channel that receives metric events. The channel is created on first call (lazy) and closed when the worker stops. Receive in a dedicated goroutine to avoid blocking the worker.
- **LastMetric()** – returns the most recent metric event and `true`, or a zero value and `false` if no event has been recorded yet. The last event is always updated, so you can poll or read on demand without using the channel.

Event kinds: **handle** (after each `Handle` call: status, duration, had message, error), **panic** (when a panic is recovered: attempt count, error), **dispatch** (when `Dispatch` is used: success or failure with error), **lifecycle** (worker started or stopped). Use the `Kind` field on `MetricEvent` to determine which payload fields are set.

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
op := smoothoperator.NewOperator(ctx)

_, err := op.AddHandler("my-worker", myHandler, smoothoperator.Config{})
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
op := smoothoperator.NewOperator(ctx)
_, _ = op.AddHandler("a", handlerA, smoothoperator.Config{})
_, _ = op.AddHandler("b", handlerB, smoothoperator.Config{})
op.StartAll()
// ...
<-op.StopAll()
```

### Handler returning None / Done / Fail

```go
func (h *MyHandler) Handle(ctx context.Context) smoothoperator.HandleResult {
    job, err := h.queue.Poll(ctx)
    if err != nil {
        return smoothoperator.Fail(err, 5*time.Second)
    }
    if job == nil {
        return smoothoperator.None(1*time.Second)
    }
    h.process(job)
    return smoothoperator.Done()
}
```

---

*Generated for package `smoothoperator` (module `github.com/diego-miranda-ng/smoothoperator`).*
