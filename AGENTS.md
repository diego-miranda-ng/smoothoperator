# AGENTS.md — smoothoperator

## Project Overview

Go library (`github.com/diego-miranda-ng/smoothoperator`) providing a worker pool abstraction for running named handlers in goroutines with coordinated lifecycle, typed message dispatch, panic recovery, and streaming metrics. Go 1.25, stdlib-only in production code; `testify` for tests.

## Architecture

Flat root package with a single `internal/` directory for test helpers and mocks. No `cmd/` directory; this is a library, not a binary.

| File | Responsibility |
|------|---------------|
| `operator.go` | `Operator` / `Dispatcher` interfaces, `operator` (unexported), `NewOperator` |
| `worker.go` | `Worker` interface, `worker` (unexported), run loops, panic recovery |
| `handler.go` | `Handler` / `DispatcherAware` interfaces, `HandleResult` constructors |
| `message.go` | `Message[T]`, `envelope`, `SendMessage` / `SendMessageWithContext` |
| `config.go` | `HandlerOption` functional options, `config` struct, `applyHandlerOptions` |
| `metrics.go` | `Metrics` interface, `metricsRecorder`, metric event types |
| `errors.go` | Sentinel errors (`ErrWorkerAlreadyExists`, `ErrWorkerNotFound`, `ErrDispatchTimeout`) |
| `doc.go` | Package-level godoc |
| `internal/worker_mock.go` | Test mocks (`HandlerMock`, `DispatcherMock`, `WorkerMock`) and handler factories |

## Code Patterns

### Exported Interfaces, Unexported Implementations

Public API is defined through interfaces (`Operator`, `Dispatcher`, `Worker`, `Handler`, `Metrics`). Concrete types (`operator`, `worker`, `metricsRecorder`) are unexported. Construction goes through `NewOperator` and `AddHandler`.

### Functional Options

- **Operator-level**: `Option func(*operator)` — e.g. `WithLogger`.
- **Per-worker**: `HandlerOption func(*config)` — e.g. `WithMessageOnly`, `WithMaxPanicAttempts`, `WithPanicBackoff`, `WithMessageBufferSize`, `WithMaxDispatchTimeout`.
- Zero values mean "use default"; normalization happens in `applyHandlerOptions`.

### Error Handling

- Sentinel errors in `errors.go`, checked with `errors.Is`.
- Wrap with `fmt.Errorf("...: %w", sentinel)`.
- Compose with `errors.Join` when combining sentinels with context errors.
- Central `operator.errorHandler` logs then returns the error.

### Concurrency

- One goroutine per worker (`worker.start` → `go func()`).
- `chan envelope` for message dispatch; `chan struct{}` for delivery confirmation and shutdown signaling.
- `sync.Mutex` on `worker`; `sync.RWMutex` on `operator` for the worker map.
- `select` with `ctx.Done()`, `time.After`, and message channels for idle/wake behavior.
- Non-blocking metric sends (`select` + `default`) to avoid blocking workers.

### Logging

- `log/slog` exclusively. Default: JSON handler to stdout.
- Per-worker child loggers via `logger.With("worker", name, ...)` baking config into default attributes.
- Levels: `Info` (lifecycle), `Debug` (handle/dispatch details), `Warn` (panics, dispatch failures), `Error` (handler failures, max panic reached).

### Metrics

- In-process channel-based streaming, per event kind (handle, panic, dispatch, lifecycle).
- Lazy channel allocation on first subscriber call.
- Best-effort delivery: drops events on full buffers.
- Channels closed when the worker stops.

### Optional Interface (`DispatcherAware`)

Handlers that need to send messages to other workers implement `DispatcherAware`. `SetDispatcher` is called once at registration. Handlers that don't need dispatch simply skip this interface.

## Testing Conventions

### Test Naming

```
Test<Subject>_When<Condition>_Should<ExpectedBehavior>
```

Examples: `TestStopAll_WhenMultipleWorkersRunning_ShouldWaitAllWorkersToStop`, `TestDispatch_WhenContextTimeout_ShouldReturnContextError`.

### Test Structure

Every test follows **Arrange / Act / Assert** with explicit comments marking each section:

```go
func TestExample_WhenCondition_ShouldBehavior(t *testing.T) {
    t.Parallel()

    // Arrange
    ...

    // Act
    ...

    // Assert
    ...
}
```

### Parallel Execution

Every test function calls `t.Parallel()` as its first statement.

### Assertion Library

- Use `require` from `github.com/stretchr/testify` for assertions that should stop the test on failure (the default).
- Use `assert` only when multiple independent checks should all run (e.g. `config_internal_test.go`).

### Black-Box vs White-Box Tests

- **Black-box** (`package smoothoperator_test`): tests in `operator_test.go`, `handler_test.go`, `message_test.go`, `metrics_test.go`, `capture_handler_test.go`. Import the package by path and use only exported API.
- **White-box** (`package smoothoperator`): tests in `*_internal_test.go` and `worker_internal_test.go`. Test unexported functions directly (e.g. `applyHandlerOptions`, `newWorker`, `sendEnvelope`).

### Test Helpers

- `internal/worker_mock.go` provides mock types and handler factories: `QuickHandler`, `IdleHandler`, `PanicHandler`, `FailHandler`, `ResultHandler`, `BlockingHandler`, `ForwarderHandler`, `NewRecordingHandler`, `NewHandlerMock`.
- `capture_handler_test.go` provides a custom `slog.Handler` (`captureHandler`) for asserting structured log output in tests.
- `worker_internal_test.go` defines a local `handlerFunc` adapter to avoid an import cycle with `internal/`.

### CI

Tests run with race detector and no caching:

```
go test ./... -count=1 -race -timeout 5m
```

## File Naming

- Source files: short domain names — `handler.go`, `worker.go`, `operator.go`, `message.go`, `metrics.go`, `config.go`, `errors.go`, `doc.go`.
- Test files: `<subject>_test.go` for black-box tests; `<subject>_internal_test.go` for white-box tests that access unexported symbols.
- Mocks: `<subject>_mock.go` in `internal/`.

## Documentation Style

- Every exported type, function, method, and constant has a godoc comment starting with the identifier name.
- Comments are full sentences ending with a period.
- `doc.go` contains the package-level overview with usage examples.
- `DOCUMENTATION.md` provides extended narrative API docs.

## Dependencies

Production code uses **only the Go standard library** (`context`, `errors`, `fmt`, `log/slog`, `os`, `sync`, `time`). The sole external dependency is `github.com/stretchr/testify` for tests.

## Adding New Code

When adding new functionality:

1. Define the interface method on the appropriate exported interface.
2. Implement on the unexported concrete type.
3. Add configuration via a new `HandlerOption` if needed (functional option pattern).
4. Add sentinel errors to `errors.go` if introducing new failure modes.
5. Write both black-box and white-box tests following the naming and structure conventions above.
6. Ensure every test calls `t.Parallel()` and uses Arrange/Act/Assert structure.
7. Add godoc comments (full sentences) to all exported symbols.

## Branch Naming

All branch names must start with a prefix that matches the type of change being made in the branch.

- `fix/` for bug fixes.
- `feature/` for new features or user-facing enhancements.
- `upkeep/` for maintenance work such as CI, tooling, dependency, or repository updates.
- `quality/` for test coverage, reliability, refactoring, observability, or other quality improvements.

After the prefix, use a short snake_case description of the work. Examples:

- `fix/dispatch_timeout_error_wrapping`
- `feature/worker_metrics_streaming`
- `upkeep/github_actions_updates`
- `quality/improve_operator_test_coverage`

## Wiki Sync

The project uses a GitHub Action (`.github/workflows/wiki-sync.yml`) to sync documentation from the `wiki/` directory to the repository's GitHub Wiki.

**Initial Setup Requirement:**
The GitHub Wiki repository must be initialized before the sync action can succeed. This requires:
1. Enabling "Wikis" in the repository settings.
2. Manually creating at least one page (e.g., a Home page) through the GitHub web interface. This creates the underlying `.wiki.git` repository that the Action pushes to.

## Pull Request Templates

Every pull request must use the correct template for the type of change being proposed and must fully complete that template before the pull request is opened or marked ready for review.

Use the template that matches the branch purpose:

- `fix.md` for `fix/` branches.
- `feature.md` for `feature/` branches.
- `upkeep.md` for `upkeep/` branches.
- `quality.md` for `quality/` branches.

The selected pull request template must be filled out completely, including the summary, validation or test details, documentation impact, and any relevant risk, compatibility, or follow-up sections.
