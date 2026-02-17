package smoothoperator

import (
	"sync"
	"time"
)

// MetricKind identifies the type of metric event.
type MetricKind string

const (
	// MetricKindHandle is emitted after each Handle call.
	MetricKindHandle MetricKind = "handle"
	// MetricKindPanic is emitted when a panic is recovered in the worker.
	MetricKindPanic MetricKind = "panic"
	// MetricKindDispatch is emitted when Dispatch is called (success or failure).
	MetricKindDispatch MetricKind = "dispatch"
	// MetricKindLifecycle is emitted when a worker starts or stops.
	MetricKindLifecycle MetricKind = "lifecycle"
)

// MetricEvent is a single metrics event. Only the fields relevant to Kind are set.
// Use Kind to determine which payload to read.
type MetricEvent struct {
	Kind   MetricKind
	Worker string
	Time   time.Time

	// Handle (MetricKindHandle): Status, Duration, HadMessage, Err.
	Status     HandleStatus
	Duration   time.Duration
	HadMessage bool
	Err        string

	// Panic (MetricKindPanic): Attempt, Err.
	PanicAttempt int
	PanicErr     string

	// Dispatch (MetricKindDispatch): Ok, Err.
	DispatchOk    bool
	DispatchError string

	// Lifecycle (MetricKindLifecycle): Event is "started" or "stopped".
	LifecycleEvent string
}

// metricsRecorder manages the lazy metrics channel and the last-event snapshot
// for a single worker. It is embedded by the worker struct and implements the
// Worker interface. All methods are safe for concurrent use.
type metricsRecorder struct {
	workerName string
	mu         sync.Mutex
	ch         chan MetricEvent
	last       *MetricEvent
}

func newMetricsRecorder(workerName string) metricsRecorder {
	return metricsRecorder{workerName: workerName}
}

// Record stores ev as the latest metric and, if the channel has been created,
// sends a copy on it. The mutex is NOT held during the channel send so the
// worker is never blocked by the recorder's internal lock.
func (m *metricsRecorder) Record(ev MetricEvent) {
	m.mu.Lock()
	evCopy := ev
	m.last = &evCopy
	ch := m.ch
	m.mu.Unlock()
	if ch != nil {
		ch <- ev
	}
}

// CloseChannel closes the metrics channel (if it was created) and nils it so no
// further sends can occur. Must be called at most once (typically from the
// worker goroutine when the run loop exits, after the final Record call).
func (m *metricsRecorder) CloseChannel() {
	m.mu.Lock()
	ch := m.ch
	m.ch = nil
	m.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

// Metrics returns a channel that receives metric events. The channel is created
// on first call with the given bufferSize and closed when the worker stops.
// Implements Worker interface. bufferSize is the capacity of the channel buffer;
// use 0 for an unbuffered channel.
func (m *metricsRecorder) Metrics(bufferSize int) <-chan MetricEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ch == nil {
		m.ch = make(chan MetricEvent, bufferSize)
	}
	return m.ch
}

// LastMetric returns the most recent metric event. Implements Worker interface.
func (m *metricsRecorder) LastMetric() (MetricEvent, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.last == nil {
		return MetricEvent{}, false
	}
	return *m.last, true
}

func (m *metricsRecorder) handleEvent(result HandleResult, duration time.Duration, hadMessage bool) MetricEvent {
	ev := MetricEvent{
		Kind:       MetricKindHandle,
		Worker:     m.workerName,
		Time:       time.Now(),
		Status:     result.Status,
		Duration:   duration,
		HadMessage: hadMessage,
	}
	if result.Err != nil {
		ev.Err = result.Err.Error()
	}
	return ev
}

func (m *metricsRecorder) lifecycleEvent(event string) MetricEvent {
	return MetricEvent{
		Kind:           MetricKindLifecycle,
		Worker:         m.workerName,
		Time:           time.Now(),
		LifecycleEvent: event,
	}
}

func (m *metricsRecorder) panicEvent(attempt int, err error) MetricEvent {
	return MetricEvent{
		Kind:         MetricKindPanic,
		Worker:       m.workerName,
		Time:         time.Now(),
		PanicAttempt: attempt,
		PanicErr:     err.Error(),
	}
}

func (m *metricsRecorder) dispatchEvent(ok bool, dispatchErr error) MetricEvent {
	ev := MetricEvent{
		Kind:       MetricKindDispatch,
		Worker:     m.workerName,
		Time:       time.Now(),
		DispatchOk: ok,
	}
	if dispatchErr != nil {
		ev.DispatchError = dispatchErr.Error()
	}
	return ev
}
