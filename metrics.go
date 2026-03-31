package smoothoperator

import (
	"sync"
	"time"
)

// Metrics provides per-worker streaming metric channels. Obtain it via
// Worker.Metrics(). Each metric kind (handle, panic, dispatch, lifecycle) has
// its own typed channel, created lazily on the first call with the given buffer
// size. Channels are closed when the worker stops. Sends are non-blocking; if
// the buffer is full the event is dropped.
type Metrics interface {
	// HandleMetrics returns a channel that receives handle metric events.
	// Created on first call with the given bufferSize; closed when the worker stops.
	HandleMetrics(bufferSize int) <-chan HandleMetricEvent
	// PanicMetrics returns a channel that receives panic metric events.
	// Created on first call with the given bufferSize; closed when the worker stops.
	PanicMetrics(bufferSize int) <-chan PanicMetricEvent
	// DispatchMetrics returns a channel that receives dispatch metric events.
	// Created on first call with the given bufferSize; closed when the worker stops.
	DispatchMetrics(bufferSize int) <-chan DispatchMetricEvent
	// LifecycleMetrics returns a channel that receives lifecycle metric events.
	// Created on first call with the given bufferSize; closed when the worker stops.
	LifecycleMetrics(bufferSize int) <-chan LifecycleMetricEvent
}

// metricBase carries the worker name and timestamp common to every metric event.
// It is embedded by all concrete event types so its fields are promoted.
type metricBase struct {
	Worker string
	Time   time.Time
}

// HandleMetricEvent is emitted after each Handle call.
type HandleMetricEvent struct {
	metricBase
	Status     HandleStatus
	Duration   time.Duration
	HadMessage bool
	Err        string
}

// PanicMetricEvent is emitted when a panic is recovered in the worker.
type PanicMetricEvent struct {
	metricBase
	Attempt int
	Err     string
}

// DispatchMetricEvent is emitted when Dispatch is called (success or failure).
type DispatchMetricEvent struct {
	metricBase
	Ok    bool
	Error string
}

// LifecycleMetricEvent is emitted when a worker starts or stops.
type LifecycleMetricEvent struct {
	metricBase
	Event string
}

// metricsRecorder manages per-kind metrics channels for a single worker.
// It is embedded by the worker struct and implements the Worker interface.
// All methods are safe for concurrent use.
type metricsRecorder struct {
	workerName  string
	mu          sync.Mutex
	handleCh    chan HandleMetricEvent
	panicCh     chan PanicMetricEvent
	dispatchCh  chan DispatchMetricEvent
	lifecycleCh chan LifecycleMetricEvent
}

// newMetricsRecorder creates a recorder for the named worker. All channels
// start as nil and are allocated lazily when a caller subscribes via one of the
// Metrics interface methods.
func newMetricsRecorder(workerName string) *metricsRecorder {
	return &metricsRecorder{workerName: workerName}
}

// HandleMetrics returns a channel that receives handle metric events. The
// channel is created on first call with the given bufferSize and closed when
// the worker stops.
func (m *metricsRecorder) HandleMetrics(bufferSize int) <-chan HandleMetricEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.handleCh == nil {
		m.handleCh = make(chan HandleMetricEvent, bufferSize)
	}
	return m.handleCh
}

// PanicMetrics returns a channel that receives panic metric events. The
// channel is created on first call with the given bufferSize and closed when
// the worker stops.
func (m *metricsRecorder) PanicMetrics(bufferSize int) <-chan PanicMetricEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.panicCh == nil {
		m.panicCh = make(chan PanicMetricEvent, bufferSize)
	}
	return m.panicCh
}

// DispatchMetrics returns a channel that receives dispatch metric events. The
// channel is created on first call with the given bufferSize and closed when
// the worker stops.
func (m *metricsRecorder) DispatchMetrics(bufferSize int) <-chan DispatchMetricEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dispatchCh == nil {
		m.dispatchCh = make(chan DispatchMetricEvent, bufferSize)
	}
	return m.dispatchCh
}

// LifecycleMetrics returns a channel that receives lifecycle metric events. The
// channel is created on first call with the given bufferSize and closed when
// the worker stops.
func (m *metricsRecorder) LifecycleMetrics(bufferSize int) <-chan LifecycleMetricEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lifecycleCh == nil {
		m.lifecycleCh = make(chan LifecycleMetricEvent, bufferSize)
	}
	return m.lifecycleCh
}

// RecordHandle sends ev on the handle channel if it exists. The send is
// non-blocking: if the buffer is full the event is dropped.
func (m *metricsRecorder) RecordHandle(ev HandleMetricEvent) {
	m.mu.Lock()
	ch := m.handleCh
	m.mu.Unlock()
	if ch != nil {
		select {
		case ch <- ev:
		default:
		}
	}
}

// RecordPanic sends ev on the panic channel if it exists. The send is
// non-blocking: if the buffer is full the event is dropped.
func (m *metricsRecorder) RecordPanic(ev PanicMetricEvent) {
	m.mu.Lock()
	ch := m.panicCh
	m.mu.Unlock()
	if ch != nil {
		select {
		case ch <- ev:
		default:
		}
	}
}

// RecordDispatch sends ev on the dispatch channel if it exists. The send is
// non-blocking: if the buffer is full the event is dropped.
func (m *metricsRecorder) RecordDispatch(ev DispatchMetricEvent) {
	m.mu.Lock()
	ch := m.dispatchCh
	m.mu.Unlock()
	if ch != nil {
		select {
		case ch <- ev:
		default:
		}
	}
}

// RecordLifecycle sends ev on the lifecycle channel if it exists. The send is
// non-blocking: if the buffer is full the event is dropped.
func (m *metricsRecorder) RecordLifecycle(ev LifecycleMetricEvent) {
	m.mu.Lock()
	ch := m.lifecycleCh
	m.mu.Unlock()
	if ch != nil {
		select {
		case ch <- ev:
		default:
		}
	}
}

// CloseChannels closes all metrics channels (if created) and nils them so no
// further sends can occur. Must be called at most once (typically from the
// worker goroutine when the run loop exits, after the final Record call).
func (m *metricsRecorder) CloseChannels() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.handleCh != nil {
		close(m.handleCh)
		m.handleCh = nil
	}
	if m.panicCh != nil {
		close(m.panicCh)
		m.panicCh = nil
	}
	if m.dispatchCh != nil {
		close(m.dispatchCh)
		m.dispatchCh = nil
	}
	if m.lifecycleCh != nil {
		close(m.lifecycleCh)
		m.lifecycleCh = nil
	}
}

// handleEvent builds a HandleMetricEvent from the given Handle result, elapsed
// duration, and whether a message was present.
func (m *metricsRecorder) handleEvent(result HandleResult, duration time.Duration, hadMessage bool) HandleMetricEvent {
	ev := HandleMetricEvent{
		metricBase: metricBase{Worker: m.workerName, Time: time.Now()},
		Status:     result.Status,
		Duration:   duration,
		HadMessage: hadMessage,
	}
	if result.Err != nil {
		ev.Err = result.Err.Error()
	}
	return ev
}

// lifecycleEvent builds a LifecycleMetricEvent with the given event name
// (e.g. "started" or "stopped").
func (m *metricsRecorder) lifecycleEvent(event string) LifecycleMetricEvent {
	return LifecycleMetricEvent{
		metricBase: metricBase{Worker: m.workerName, Time: time.Now()},
		Event:      event,
	}
}

// panicEvent builds a PanicMetricEvent with the cumulative attempt count and
// the recovered panic converted to an error.
func (m *metricsRecorder) panicEvent(attempt int, err error) PanicMetricEvent {
	return PanicMetricEvent{
		metricBase: metricBase{Worker: m.workerName, Time: time.Now()},
		Attempt:    attempt,
		Err:        err.Error(),
	}
}

// dispatchEvent builds a DispatchMetricEvent indicating whether the dispatch
// succeeded and, on failure, the associated error.
func (m *metricsRecorder) dispatchEvent(ok bool, dispatchErr error) DispatchMetricEvent {
	ev := DispatchMetricEvent{
		metricBase: metricBase{Worker: m.workerName, Time: time.Now()},
		Ok:         ok,
	}
	if dispatchErr != nil {
		ev.Error = dispatchErr.Error()
	}
	return ev
}
