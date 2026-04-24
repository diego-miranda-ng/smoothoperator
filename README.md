# smoothoperator

A Go library for running multiple named workers (goroutines) managed by a central operator. Register handlers by name, start and stop them individually or all at once, send typed messages between workers, and observe execution via streaming metrics channels. Each worker runs a `Handler` in a loop (or on-demand when messages arrive) with built-in panic recovery, configurable idle backoff, and structured logging.

## Install

```bash
go get github.com/diego-miranda-ng/smoothoperator
```

Requires Go 1.25+.

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"time"

	so "github.com/diego-miranda-ng/smoothoperator"
)

type printer struct{}

func (printer) Handle(ctx context.Context, msg any) so.HandleResult {
	if msg == nil {
		return so.None(1 * time.Second)
	}
	m := msg.(so.Message[string])
	fmt.Println("received:", m.Data)
	return so.Done()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	op := so.NewOperator(ctx)

	_, _ = op.AddHandler("printer", printer{}, so.WithMessageOnly(true))
	_ = op.StartAll()

	so.SendMessage(op, "printer", "hello world")

	time.Sleep(100 * time.Millisecond)
	<-op.StopAll()
}
```

## Features

- **Named workers** -- register, start, stop, and remove workers by name.
- **Polling and message-only modes** -- workers can run in a continuous loop or activate only when a message arrives.
- **Typed messages** -- send `Message[T]` to workers with `SendMessage` / `SendMessageWithContext` for type-safe payloads.
- **Dispatch with results** -- `Dispatch` returns channels for delivery confirmation and handler results.
- **Cross-worker communication** -- handlers implementing `DispatcherAware` receive a `Dispatcher` to send messages to other workers.
- **Panic recovery** -- workers recover from panics, log them, back off, and continue. Configurable max attempts.
- **Streaming metrics** -- per-worker channels for handle, panic, dispatch, and lifecycle events.
- **Structured logging** -- uses `log/slog` with per-worker child loggers. Bring your own logger with `WithLogger`.

## Handler results

| Constructor | When to use | Worker behavior |
|---|---|---|
| `None(idle)` | No work available | Sleeps for `idle`, then calls `Handle` again |
| `Done()` | Work completed | Calls `Handle` again immediately |
| `DoneWithResult(v)` | Work completed, return value to dispatcher | Same as `Done`, plus sends `v` on the result channel |
| `Fail(err, idle)` | Handler error | Logs `err`, sleeps for `idle`, then retries |

## Configuration

Options passed to `AddHandler` to configure per-worker behavior:

| Option | Default | Description |
|---|---|---|
| `WithMessageOnly(true)` | `false` | Only call `Handle` when a message arrives (skip polling loop) |
| `WithMessageBufferSize(n)` | `1` | Capacity of the incoming message channel |
| `WithMaxDispatchTimeout(d)` | `0` (no limit) | Max time `Dispatch` waits to send a message to this worker |
| `WithMaxPanicAttempts(n)` | `0` (unlimited) | Stop the worker after `n` panic recoveries |
| `WithPanicBackoff(d)` | `1s` | Sleep duration after a panic before retrying |

## Documentation

For the full API reference, all types, and detailed copy-paste examples see **[the wiki](https://github.com/diego-miranda-ng/smoothoperator/wiki)**.

> **Note:** The GitHub Wiki must be enabled in the repository settings, and at least one page must be created manually via the GitHub UI to initialize the wiki repository before the automated sync action can work.

You can also browse with `go doc`:

```bash
go doc github.com/diego-miranda-ng/smoothoperator
go doc github.com/diego-miranda-ng/smoothoperator.Operator
go doc github.com/diego-miranda-ng/smoothoperator.Handler
```

## Project structure

- **Root package** -- public API: `Operator`, `Dispatcher`, `Worker`, `Handler`, `HandleResult`, `Message[T]`, `Metrics`, metric event types, sentinel errors.
- **internal/** -- test helpers and mocks (not part of the public API).

## Development

### Prerequisites

- [Go](https://go.dev/dl/) 1.25+
- [Docker](https://www.docker.com/get-started) (optional, for dev container)

### Commands

```bash
go build ./...
go test ./...
```

This project supports Docker and VS Code Dev Containers for a consistent environment; see [.devcontainer](.devcontainer) and [.vscode](.vscode).
