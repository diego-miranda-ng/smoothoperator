# smoothoperator -- Package Documentation

**Module:** `github.com/diego-miranda-ng/smoothoperator`
**Package:** `smoothoperator`

---

## Table of contents

1. [Overview](#overview)
2. [Installation](#installation)
3. [Core concepts](#core-concepts)
4. [API reference](#api-reference)
   - [NewOperator](#newoperator)
   - [Option and WithLogger](#option-and-withlogger)
   - [Operator interface](#operator-interface)
   - [Dispatcher interface](#dispatcher-interface)
   - [Handler interface](#handler-interface)
   - [DispatcherAware interface](#dispatcheraware-interface)
   - [HandleResult and HandleStatus](#handleresult-and-handlestatus)
   - [HandleResult constructors](#handleresult-constructors)
   - [HandlerOption functions](#handleroption-functions)
   - [Message and SendMessage](#message-and-sendmessage)
   - [Worker interface](#worker-interface)
   - [Status](#status)
   - [Metrics interface](#metrics-interface)
   - [Metric event types](#metric-event-types)
   - [Sentinel errors](#sentinel-errors)
5. [Usage examples](#usage-examples)
   - [Basic polling worker](#example-1-basic-polling-worker)
   - [Message-only worker with typed messages](#example-2-message-only-worker-with-typed-messages)
   - [Multiple workers with StartAll and StopAll](#example-3-multiple-workers-with-startall-and-stopall)
   - [Dispatch with results](#example-4-dispatch-with-results)
   - [Cross-worker communication via DispatcherAware](#example-5-cross-worker-communication-via-dispatcheraware)
   - [Observing worker metrics](#example-6-observing-worker-metrics)
   - [Custom logger with WithLogger](#example-7-custom-logger-with-withlogger)
   - [Panic recovery and max attempts](#example-8-panic-recovery-and-max-attempts)
   - [Removing a handler at runtime](#example-9-removing-a-handler-at-runtime)

---

## Overview

`smoothoperator` is a named worker pool for Go. You create an **Operator**, register **Handlers** by name, then start and stop workers individually or all at once. Each worker runs its handler in a goroutine, calling `Handle` repeatedly until stopped.

Key capabilities:

- **Named workers** -- start, stop, query status, and remove workers by name.
- **Two execution modes** -- polling (continuous loop) or message-only (handler fires only when a message arrives).
- **Typed messaging** -- send `Message[T]` payloads to workers; receive delivery confirmation and handler results through channels.
- **Cross-worker dispatch** -- handlers implementing `DispatcherAware` can send messages to other workers.
- **Panic recovery** -- workers recover from panics, log them, back off, and continue (with optional max attempts).
- **Streaming metrics** -- four per-worker metric channels (handle, panic, dispatch, lifecycle) for observability.
- **Structured logging** -- `log/slog` with automatic per-worker child loggers.

---

## Installation

```bash
go get github.com/diego-miranda-ng/smoothoperator
```

Requires Go 1.25+.

---

## Core concepts

| Concept | Description |
|---|---|
| **Operator** | Central manager. Registers handlers, starts/stops workers, dispatches messages. Implements both `Operator` and `Dispatcher` interfaces. |
| **Handler** | Your business logic. Implements `Handle(ctx, msg) HandleResult`. Called repeatedly by the worker. |
| **Worker** | A goroutine running a handler. Created and managed by the operator -- not constructed directly. Exposes `Name()`, `Status()`, and `Metrics()`. |
| **Dispatcher** | Sends messages to workers by name. The operator implements this interface. |
| **Message[T]** | A generic wrapper for typed payloads sent to workers via `SendMessage` or `Dispatch`. |
| **HandleResult** | Returned by `Handle` to tell the worker what to do next: continue, idle, or report failure. Built with `None`, `Done`, `DoneWithResult`, or `Fail`. |
| **Metrics** | Per-worker streaming channels for handle, panic, dispatch, and lifecycle events. |

---

## API reference

### NewOperator

```go
func NewOperator(ctx context.Context, opts ...Option) Operator
```

Creates an `Operator` that uses `ctx` for worker lifecycle. Workers started via this operator run until `ctx` is cancelled or `Stop` / `StopAll` is called.

**Parameters:**

- `ctx` -- context shared by all workers. Cancelling it stops every worker.
- `opts` -- optional configuration (e.g. `WithLogger`).

**Returns:** an `Operator` (concrete type is unexported).

**Logging:** By default, a JSON `slog.Handler` writing to `os.Stdout` is used. Each worker gets a child logger with a `"worker"` attribute set to its name, so every log line is automatically attributed.

---

### Option and WithLogger

```go
type Option func(*operator)

func WithLogger(logger *slog.Logger) Option
```

`Option` configures an operator at creation time.

`WithLogger` sets the logger for the operator and all its workers. If `logger` is `nil`, the default JSON logger is used.

---

### Operator interface

```go
type Operator interface {
    Dispatcher
    AddHandler(name string, handler Handler, opts ...HandlerOption) (Worker, error)
    Start(name string) error
    StartAll() error
    Stop(name string) (chan struct{}, error)
    StopAll() chan struct{}
    RemoveHandler(name string) error
    Status(name string) (Status, error)
    Worker(name string) (Worker, error)
}
```

All methods are safe for concurrent use.

#### AddHandler

```go
AddHandler(name string, handler Handler, opts ...HandlerOption) (Worker, error)
```

Registers a handler under the given name. The worker is **not started** -- call `Start` or `StartAll` to run it.

If the handler implements `DispatcherAware`, `SetDispatcher` is called immediately with the operator's `Dispatcher`.

**Parameters:**

- `name` -- unique identifier for the worker.
- `handler` -- your `Handler` implementation.
- `opts` -- optional `HandlerOption` values (e.g. `WithMaxPanicAttempts(3)`, `WithMessageOnly(true)`). Omit for defaults.

**Returns:**

- `Worker` -- interface for reading metrics. `nil` when an error occurs.
- `error` -- non-nil if the name is already registered (`ErrWorkerAlreadyExists`).

#### Start

```go
Start(name string) error
```

Starts the worker with the given name. If the worker is already running, this is a no-op and returns `nil`.

**Returns:** `error` -- non-nil if no worker is registered under `name` (`ErrWorkerNotFound`).

#### StartAll

```go
StartAll() error
```

Starts every registered worker. If any `Start` fails, returns that error and does not start remaining workers.

#### Stop

```go
Stop(name string) (chan struct{}, error)
```

Stops the named worker and returns a channel that closes when the worker goroutine has exited.

**Returns:**

- `chan struct{}` -- closes when the worker has fully stopped. `nil` if the name is not found.
- `error` -- non-nil if the name is not registered (`ErrWorkerNotFound`).

**Typical usage:**

```go
ch, err := op.Stop("my-worker")
if err != nil { /* handle */ }
<-ch // block until the worker is done
```

#### StopAll

```go
StopAll() chan struct{}
```

Stops all registered workers and returns a channel that closes when every worker has stopped. Always non-nil.

**Typical usage:**

```go
<-op.StopAll()
```

#### RemoveHandler

```go
RemoveHandler(name string) error
```

Stops the named worker (if running), waits for it to finish, and removes it from the operator. After this call the name can be reused with `AddHandler`.

**Returns:** `error` -- non-nil if the name is not registered (`ErrWorkerNotFound`).

#### Status

```go
Status(name string) (Status, error)
```

Returns the current status of the named worker (`StatusRunning` or `StatusStopped`).

**Returns:** `error` -- non-nil if the name is not registered (`ErrWorkerNotFound`).

#### Worker (method)

```go
Worker(name string) (Worker, error)
```

Returns the `Worker` interface for the named worker, used to read metrics.

**Returns:** `error` -- non-nil if the name is not registered (`ErrWorkerNotFound`).

---

### Dispatcher interface

```go
type Dispatcher interface {
    Dispatch(ctx context.Context, name string, msg any) (delivered <-chan struct{}, result <-chan any, err error)
}
```

The operator implements `Dispatcher`. You can also obtain it inside a handler via `DispatcherAware`.

#### Dispatch

```go
Dispatch(ctx context.Context, name string, msg any) (delivered <-chan struct{}, result <-chan any, err error)
```

Sends a message to the worker with the given name.

**Parameters:**

- `ctx` -- controls the send timeout. If the worker's buffer is full, `Dispatch` blocks until space is available or `ctx` is done.
- `name` -- target worker name.
- `msg` -- the payload passed to `Handle` as the `msg` parameter.

**Returns:**

- `delivered` -- closes once the handler has received (dequeued) the message.
- `result` -- receives the value from `HandleResult.Result` when the handler finishes, then closes.
- `err` -- non-nil if the worker is not found (`ErrWorkerNotFound`) or the send timed out (`ErrDispatchTimeout` joined with the context error).

When `WithMaxDispatchTimeout` is set for the target worker, the send is also bounded by that duration (in addition to `ctx`).

---

### Handler interface

```go
type Handler interface {
    Handle(ctx context.Context, msg any) HandleResult
}
```

Implement `Handle` to perform one unit of work. The worker calls it repeatedly until stopped.

**Parameters:**

- `ctx` -- the worker's context. Check `ctx.Done()` for cancellation.
- `msg` -- the message sent via `Dispatch` / `SendMessage`, or `nil` when no message is pending (in polling mode).

**Returns:** a `HandleResult` built with `None`, `Done`, `DoneWithResult`, or `Fail`.

---

### DispatcherAware interface

```go
type DispatcherAware interface {
    SetDispatcher(disp Dispatcher)
}
```

Optional interface. If your handler implements it, `SetDispatcher` is called once when the handler is registered via `AddHandler`. Store the `Dispatcher` and use it inside `Handle` to send messages to other workers.

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
    Result       any
}
```

**Fields:**

| Field | Description |
|---|---|
| `Status` | Outcome of this `Handle` invocation. |
| `IdleDuration` | When `Status` is `None` or `Fail`, the worker sleeps this long before the next `Handle`. Ignored for `Done`. |
| `Err` | Set when `Status` is `Fail`. Logged by the worker. |
| `Result` | Optional data returned to the caller of `Dispatch`. Sent on the result channel after `Handle` finishes. |

**Status values:**

| Value | Meaning |
|---|---|
| `HandleStatusNone` (`"none"`) | No work available. Worker sleeps for `IdleDuration`. |
| `HandleStatusDone` (`"done"`) | Work processed. Worker continues immediately. |
| `HandleStatusFail` (`"fail"`) | Handler failed. `Err` is logged. Worker sleeps for `IdleDuration`. |

---

### HandleResult constructors

#### None

```go
func None(idle time.Duration) HandleResult
```

No work was available. The worker sleeps for `idle` before the next `Handle`. Pass `0` to poll without sleeping.

#### Done

```go
func Done() HandleResult
```

Work completed. The worker proceeds to the next `Handle` immediately.

#### DoneWithResult

```go
func DoneWithResult(result any) HandleResult
```

Work completed and the handler wants to return data to the caller of `Dispatch`. The `result` is sent on the result channel returned by `Dispatch`.

#### Fail

```go
func Fail(err error, idle time.Duration) HandleResult
```

Handler failed. `err` is stored and logged. `idle` is the backoff before the next attempt. Pass `0` to retry immediately.

---

### HandlerOption functions

Options passed to `AddHandler` to configure per-worker behavior. Omit them for defaults.

```go
type HandlerOption func(*config)
```

| Function | Default | Description |
|---|---|---|
| `WithMessageOnly(b bool)` | `false` | When `true`, `Handle` is only called when a message arrives (no polling loop). |
| `WithMessageBufferSize(n int)` | `1` | Capacity of the worker's incoming message channel. Values <= 0 are normalized to 1. |
| `WithMaxDispatchTimeout(d time.Duration)` | `0` (no limit) | Max wait time when sending a message to this worker. Combined with the caller's context. |
| `WithMaxPanicAttempts(n int)` | `0` (unlimited) | Stop the worker after `n` panic recoveries. |
| `WithPanicBackoff(d time.Duration)` | `1s` | Sleep duration after a panic before retrying. Values <= 0 are normalized to 1s. |

---

### Message and SendMessage

#### Message[T]

```go
type Message[T any] struct {
    Data T
}
```

Generic wrapper for typed payloads. When you call `SendMessage[string](op, "w", "hello")`, the handler receives a `Message[string]{Data: "hello"}` as the `msg` parameter.

#### SendMessage

```go
func SendMessage[T any](op Operator, name string, data T) (<-chan struct{}, <-chan any, error)
```

Sends a typed message to the named worker using `context.Background()` (blocks until buffer space is available). Equivalent to `op.Dispatch(context.Background(), name, Message[T]{Data: data})`.

**Returns:** same as `Dispatch` -- delivered channel, result channel, error.

#### SendMessageWithContext

```go
func SendMessageWithContext[T any](ctx context.Context, op Operator, name string, data T) (<-chan struct{}, <-chan any, error)
```

Same as `SendMessage` but with a caller-provided context for timeout/cancellation.

---

### Worker interface

```go
type Worker interface {
    Name() string
    Status() Status
    Metrics() Metrics
}
```

Obtained from `AddHandler` or `Operator.Worker(name)`. The concrete type is unexported.

| Method | Description |
|---|---|
| `Name()` | Returns the worker's registered name. |
| `Status()` | Returns `StatusRunning` or `StatusStopped`. |
| `Metrics()` | Returns the `Metrics` interface for subscribing to metric event channels. |

---

### Status

```go
type Status string

const (
    StatusRunning Status = "running"
    StatusStopped Status = "stopped"
)
```

---

### Metrics interface

```go
type Metrics interface {
    HandleMetrics(bufferSize int) <-chan HandleMetricEvent
    PanicMetrics(bufferSize int) <-chan PanicMetricEvent
    DispatchMetrics(bufferSize int) <-chan DispatchMetricEvent
    LifecycleMetrics(bufferSize int) <-chan LifecycleMetricEvent
}
```

Each method returns a channel that receives the corresponding event type. Channels are created lazily on first call with the given `bufferSize` and closed when the worker stops.

Sends are **non-blocking**: if the channel buffer is full, the event is dropped. Use a large enough `bufferSize` or drain the channel in a dedicated goroutine.

| Method | Event type | When emitted |
|---|---|---|
| `HandleMetrics(n)` | `HandleMetricEvent` | After each `Handle` call |
| `PanicMetrics(n)` | `PanicMetricEvent` | When a panic is recovered |
| `DispatchMetrics(n)` | `DispatchMetricEvent` | When `Dispatch` sends (or fails to send) a message to this worker |
| `LifecycleMetrics(n)` | `LifecycleMetricEvent` | When the worker starts or stops |

---

### Metric event types

All event types embed an unexported `metricBase` that provides two promoted fields:

| Field | Type | Description |
|---|---|---|
| `Worker` | `string` | Name of the worker that emitted the event. |
| `Time` | `time.Time` | Timestamp when the event was recorded. |

#### HandleMetricEvent

```go
type HandleMetricEvent struct {
    Worker     string        // (promoted) worker name
    Time       time.Time     // (promoted) event timestamp
    Status     HandleStatus  // outcome of the Handle call
    Duration   time.Duration // how long Handle took
    HadMessage bool          // true if a message was present
    Err        string        // error string (empty if no error)
}
```

#### PanicMetricEvent

```go
type PanicMetricEvent struct {
    Worker  string    // (promoted) worker name
    Time    time.Time // (promoted) event timestamp
    Attempt int       // cumulative panic count for this worker
    Err     string    // panic value as string
}
```

#### DispatchMetricEvent

```go
type DispatchMetricEvent struct {
    Worker string    // (promoted) worker name
    Time   time.Time // (promoted) event timestamp
    Ok     bool      // true if the message was sent successfully
    Error  string    // error string on failure (empty on success)
}
```

#### LifecycleMetricEvent

```go
type LifecycleMetricEvent struct {
    Worker string    // (promoted) worker name
    Time   time.Time // (promoted) event timestamp
    Event  string    // "started" or "stopped"
}
```

---

### Sentinel errors

Use `errors.Is` to check error types:

```go
var (
    ErrWorkerAlreadyExists = errors.New("worker already exists")
    ErrWorkerNotFound      = errors.New("worker not found")
    ErrDispatchTimeout     = errors.New("dispatch timeout")
)
```

| Error | Returned by |
|---|---|
| `ErrWorkerAlreadyExists` | `AddHandler` when the name is already registered. |
| `ErrWorkerNotFound` | `Start`, `Stop`, `RemoveHandler`, `Status`, `Worker`, `Dispatch` when the name is not registered. |
| `ErrDispatchTimeout` | `Dispatch` when the send times out (joined with the underlying context error, e.g. `context.DeadlineExceeded`). |

```go
_, err := op.AddHandler("w", h)
if errors.Is(err, smoothoperator.ErrWorkerAlreadyExists) {
    // name already in use
}

err = op.Start("missing")
if errors.Is(err, smoothoperator.ErrWorkerNotFound) {
    // no such worker
}

_, _, err = op.Dispatch(ctx, "w", payload)
if errors.Is(err, smoothoperator.ErrDispatchTimeout) {
    // send timed out
}
```

---

## Usage examples

Each example below is a complete, self-contained `main.go` that you can copy-paste and run.

---

### Example 1: Basic polling worker

A worker that runs in a continuous loop. When there is no work, it idles for 1 second.

```go
package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	so "github.com/diego-miranda-ng/smoothoperator"
)

type poller struct{}

func (poller) Handle(ctx context.Context, msg any) so.HandleResult {
	// Simulate checking for work.
	if rand.Intn(3) == 0 {
		fmt.Println("no work available, idling...")
		return so.None(1 * time.Second)
	}
	fmt.Println("processing job...")
	time.Sleep(200 * time.Millisecond)
	return so.Done()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := so.NewOperator(ctx)

	_, err := op.AddHandler("poller", poller{})
	if err != nil {
		panic(err)
	}

	if err := op.Start("poller"); err != nil {
		panic(err)
	}

	// Let it run for 5 seconds.
	time.Sleep(5 * time.Second)

	ch, _ := op.Stop("poller")
	<-ch
	fmt.Println("poller stopped")
}
```

---

### Example 2: Message-only worker with typed messages

A worker that only runs when it receives a message. Uses `SendMessage` for type-safe dispatch.

```go
package main

import (
	"context"
	"fmt"
	"time"

	so "github.com/diego-miranda-ng/smoothoperator"
)

type Order struct {
	ID    int
	Item  string
	Price float64
}

type orderHandler struct{}

func (orderHandler) Handle(ctx context.Context, msg any) so.HandleResult {
	order := msg.(so.Message[Order])
	fmt.Printf("processing order #%d: %s ($%.2f)\n", order.Data.ID, order.Data.Item, order.Data.Price)
	return so.Done()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := so.NewOperator(ctx)

	_, err := op.AddHandler("orders", orderHandler{}, so.WithMessageOnly(true))
	if err != nil {
		panic(err)
	}
	_ = op.StartAll()

	// Send a few orders.
	orders := []Order{
		{ID: 1, Item: "Keyboard", Price: 79.99},
		{ID: 2, Item: "Mouse", Price: 29.99},
		{ID: 3, Item: "Monitor", Price: 349.00},
	}
	for _, o := range orders {
		delivered, _, err := so.SendMessage(op, "orders", o)
		if err != nil {
			panic(err)
		}
		<-delivered // wait for the handler to pick up the message
		fmt.Printf("order #%d delivered to handler\n", o.ID)
	}

	time.Sleep(100 * time.Millisecond)
	<-op.StopAll()
}
```

---

### Example 3: Multiple workers with StartAll and StopAll

Register several workers and manage them as a group.

```go
package main

import (
	"context"
	"fmt"
	"time"

	so "github.com/diego-miranda-ng/smoothoperator"
)

type namedWorker struct{ label string }

func (w namedWorker) Handle(ctx context.Context, msg any) so.HandleResult {
	fmt.Printf("[%s] tick\n", w.label)
	return so.None(1 * time.Second)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := so.NewOperator(ctx)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		_, err := op.AddHandler(name, namedWorker{label: name})
		if err != nil {
			panic(err)
		}
	}

	if err := op.StartAll(); err != nil {
		panic(err)
	}

	// Check individual statuses.
	for _, name := range []string{"alpha", "beta", "gamma"} {
		status, _ := op.Status(name)
		fmt.Printf("%s is %s\n", name, status) // "running"
	}

	time.Sleep(3 * time.Second)

	<-op.StopAll()
	fmt.Println("all workers stopped")
}
```

---

### Example 4: Dispatch with results

Use `Dispatch` (or `SendMessage`) and read the result channel to get data back from the handler.

```go
package main

import (
	"context"
	"fmt"

	so "github.com/diego-miranda-ng/smoothoperator"
)

type calculator struct{}

func (calculator) Handle(ctx context.Context, msg any) so.HandleResult {
	m := msg.(so.Message[int])
	squared := m.Data * m.Data
	return so.DoneWithResult(squared)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := so.NewOperator(ctx)

	_, err := op.AddHandler("calc", calculator{}, so.WithMessageOnly(true))
	if err != nil {
		panic(err)
	}
	_ = op.Start("calc")

	// Send a value and wait for the result.
	delivered, resultCh, err := so.SendMessage(op, "calc", 7)
	if err != nil {
		panic(err)
	}
	<-delivered
	result := <-resultCh
	fmt.Printf("7 squared = %d\n", result) // 7 squared = 49

	// Send another value.
	_, resultCh, _ = so.SendMessage(op, "calc", 12)
	fmt.Printf("12 squared = %d\n", <-resultCh) // 12 squared = 144

	<-op.StopAll()
}
```

---

### Example 5: Cross-worker communication via DispatcherAware

A handler that sends messages to another worker. Implement `DispatcherAware` to receive the `Dispatcher` at registration time.

```go
package main

import (
	"context"
	"fmt"
	"time"

	so "github.com/diego-miranda-ng/smoothoperator"
)

// logger receives string messages and prints them.
type logger struct{}

func (logger) Handle(ctx context.Context, msg any) so.HandleResult {
	m := msg.(so.Message[string])
	fmt.Println("[LOG]", m.Data)
	return so.Done()
}

// producer generates work and forwards log entries to the "logger" worker.
type producer struct {
	disp so.Dispatcher
}

func (p *producer) SetDispatcher(disp so.Dispatcher) {
	p.disp = disp
}

func (p *producer) Handle(ctx context.Context, msg any) so.HandleResult {
	// Simulate producing work.
	logMsg := fmt.Sprintf("produced item at %s", time.Now().Format(time.RFC3339))

	// Send a log message to the logger worker.
	so.SendMessageWithContext(ctx, p.disp.(so.Operator), "logger", logMsg)

	return so.None(1 * time.Second)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := so.NewOperator(ctx)

	_, _ = op.AddHandler("logger", logger{}, so.WithMessageOnly(true))
	_, _ = op.AddHandler("producer", &producer{}) // pointer so SetDispatcher can store the value

	_ = op.StartAll()

	time.Sleep(5 * time.Second)
	<-op.StopAll()
}
```

---

### Example 6: Observing worker metrics

Subscribe to handle and lifecycle metric channels to monitor worker execution.

```go
package main

import (
	"context"
	"fmt"
	"time"

	so "github.com/diego-miranda-ng/smoothoperator"
)

type ticker struct{}

func (ticker) Handle(ctx context.Context, msg any) so.HandleResult {
	return so.None(500 * time.Millisecond)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := so.NewOperator(ctx)

	w, _ := op.AddHandler("ticker", ticker{})

	// Subscribe to metric channels before starting the worker.
	handleCh := w.Metrics().HandleMetrics(10)
	lifecycleCh := w.Metrics().LifecycleMetrics(10)

	_ = op.Start("ticker")

	// Read metrics in a goroutine.
	go func() {
		for ev := range lifecycleCh {
			fmt.Printf("[lifecycle] worker=%s event=%s time=%s\n", ev.Worker, ev.Event, ev.Time.Format(time.RFC3339))
		}
	}()

	go func() {
		for ev := range handleCh {
			fmt.Printf("[handle] worker=%s status=%s duration=%s had_message=%v\n",
				ev.Worker, ev.Status, ev.Duration, ev.HadMessage)
		}
	}()

	time.Sleep(3 * time.Second)
	<-op.StopAll()

	// Give goroutines a moment to drain closed channels.
	time.Sleep(100 * time.Millisecond)
	fmt.Println("done")
}
```

---

### Example 7: Custom logger with WithLogger

Provide your own `slog.Logger` for custom output format or destination.

```go
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	so "github.com/diego-miranda-ng/smoothoperator"
)

type quietWorker struct{}

func (quietWorker) Handle(ctx context.Context, msg any) so.HandleResult {
	return so.None(1 * time.Second)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use a text handler writing to stderr at warn level.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	op := so.NewOperator(ctx, so.WithLogger(logger))

	_, _ = op.AddHandler("quiet", quietWorker{})
	_ = op.StartAll()

	time.Sleep(3 * time.Second)
	<-op.StopAll()
}
```

---

### Example 8: Panic recovery and max attempts

Configure a worker to tolerate a limited number of panics before stopping itself.

```go
package main

import (
	"context"
	"fmt"
	"time"

	so "github.com/diego-miranda-ng/smoothoperator"
)

type unstable struct{ calls int }

func (u *unstable) Handle(ctx context.Context, msg any) so.HandleResult {
	u.calls++
	if u.calls%2 == 0 {
		panic("something went wrong")
	}
	fmt.Printf("call #%d OK\n", u.calls)
	return so.None(200 * time.Millisecond)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := so.NewOperator(ctx)

	w, _ := op.AddHandler("unstable", &unstable{},
		so.WithMaxPanicAttempts(3),
		so.WithPanicBackoff(500*time.Millisecond),
	)

	// Subscribe to panic metrics.
	panicCh := w.Metrics().PanicMetrics(10)
	go func() {
		for ev := range panicCh {
			fmt.Printf("[panic] attempt=%d err=%s\n", ev.Attempt, ev.Err)
		}
	}()

	_ = op.Start("unstable")

	// The worker will stop itself after 3 panics.
	time.Sleep(10 * time.Second)

	status, _ := op.Status("unstable")
	fmt.Println("final status:", status) // "stopped"
}
```

---

### Example 9: Removing a handler at runtime

Use `RemoveHandler` to stop a worker and unregister it so the name can be reused.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	so "github.com/diego-miranda-ng/smoothoperator"
)

type temporary struct{}

func (temporary) Handle(ctx context.Context, msg any) so.HandleResult {
	fmt.Println("working...")
	return so.None(500 * time.Millisecond)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := so.NewOperator(ctx)

	_, _ = op.AddHandler("temp", temporary{})
	_ = op.Start("temp")
	time.Sleep(2 * time.Second)

	// Remove the handler (stops it and unregisters).
	if err := op.RemoveHandler("temp"); err != nil {
		panic(err)
	}
	fmt.Println("handler removed")

	// The name is now available again.
	_, err := op.AddHandler("temp", temporary{})
	if err != nil {
		panic(err)
	}
	fmt.Println("handler re-registered")

	// Trying to remove a non-existent name returns ErrWorkerNotFound.
	err = op.RemoveHandler("nonexistent")
	if errors.Is(err, so.ErrWorkerNotFound) {
		fmt.Println("correctly got ErrWorkerNotFound")
	}

	<-op.StopAll()
}
```

---

*Generated for package `smoothoperator` -- module `github.com/diego-miranda-ng/smoothoperator`.*
