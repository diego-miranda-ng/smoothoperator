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

    op := workermanager.NewOperator(ctx)

    op.AddHandler("my-worker", myHandler{})
    op.StartAll()

    // ... run until shutdown ...
    cancel()
    <-op.StopAll()
}

type myHandler struct{}

func (myHandler) Handle(ctx context.Context) workermanager.HandleResult {
    // Do work; return workermanager.Done(), workermanager.None(idle), or workermanager.Fail(err, idle).
    return workermanager.Done()
}
```

- **`Handler`** implements `Handle(ctx) HandleResult`. Return `Done()`, `None(idleDuration)`, or `Fail(err, idleDuration)`.
- **`NewOperator(ctx)`** creates an operator. Use `AddHandler(name, handler)`, then `Start(name)` or `StartAll()`, and `Stop(name)` / `StopAll()` for shutdown.

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
