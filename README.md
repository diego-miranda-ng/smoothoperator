# smoothoperator

A small Go library to run multiple named workers (goroutines) with a shared context. Each worker runs a `Handler` in a loop until the context is cancelled; handlers can report "no work" and sleep for a configurable duration to avoid busy-looping.

## Install

Use the library in your module:

```bash
go get github.com/diego-miranda-ng/smoothoperator
```

## Usage

```go
package main

import (
    "context"
    "github.com/diego-miranda-ng/smoothoperator"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    op := smoothoperator.NewOperator(ctx)
    // Optional: op := smoothoperator.NewOperator(ctx, smoothoperator.WithLogger(myLogger))

    _, _ = op.AddHandler("my-worker", myHandler{}, smoothoperator.Config{})
    op.StartAll()

    // ... run until shutdown ...
    cancel()
    <-op.StopAll()
}

type myHandler struct{}

func (myHandler) Handle(ctx context.Context) smoothoperator.HandleResult {
    // Do work; return smoothoperator.Done(), smoothoperator.None(idle), or smoothoperator.Fail(err, idle).
    return smoothoperator.Done()
}
```

- **`Handler`** implements `Handle(ctx) HandleResult`. Return `Done()`, `None(idleDuration)`, or `Fail(err, idleDuration)`.
- **`NewOperator(ctx)`** or **`NewOperator(ctx, smoothoperator.WithLogger(logger))`** creates an operator. Without options, a default JSON logger writing to `os.Stdout` is used; with `WithLogger` you can plug your own logger (logs form a tree: operator → each worker with a `"worker"` attribute). Use `AddHandler(name, handler, config)` (returns `Worker, error`), then `Start(name)` or `StartAll()`, and `Stop(name)` / `StopAll()` for shutdown. Use `Status(name)` to get a worker’s status. Pass `Config{}` for defaults, or set `Config{MaxPanicAttempts: n}` to stop a worker after n panic recoveries.

## Documentation

For a full reference of all types and methods, see **[DOCUMENTATION.md](DOCUMENTATION.md)**. You can also use the standard godoc from the package directory:

```bash
go doc github.com/diego-miranda-ng/smoothoperator
go doc github.com/diego-miranda-ng/smoothoperator.Operator
```

## Project structure

- **Root package** – public API: `Operator`, `Worker`, `Handler`, `HandleResult`, `NewOperator`, etc.
- **internal** – test helpers and mocks (not part of the public API).

## Development

### Prerequisites

- [Docker](https://www.docker.com/get-started) (optional, for dev container)
- [Go](https://go.dev/dl/) 1.25+

### Commands

```bash
go build ./...
go test ./...
```

This project can use Docker and VS Code Dev Containers for a consistent environment; see [.devcontainer](.devcontainer) and [.vscode](.vscode).
