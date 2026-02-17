package smoothoperator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyHandlerOptions_NoOptions_ReturnsDefaults(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions()

	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize, "messageBufferSize should be default")
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff, "panicBackoff should be default")
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMaxPanicAttempts_SetsField(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(WithMaxPanicAttempts(3))

	assert.Equal(t, 3, cfg.maxPanicAttempts)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
}

func TestApplyHandlerOptions_WithMaxPanicAttemptsZero_KeepsZero(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(WithMaxPanicAttempts(0))

	assert.Equal(t, 0, cfg.maxPanicAttempts)
}

func TestApplyHandlerOptions_WithPanicBackoff_SetsField(t *testing.T) {
	t.Parallel()
	d := 2 * time.Second
	cfg := applyHandlerOptions(WithPanicBackoff(d))

	assert.Equal(t, d, cfg.panicBackoff)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
}

func TestApplyHandlerOptions_WithPanicBackoffZero_NormalizedToDefault(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(WithPanicBackoff(0))

	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
}

func TestApplyHandlerOptions_WithMessageBufferSize_SetsField(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(WithMessageBufferSize(10))

	assert.Equal(t, 10, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
}

func TestApplyHandlerOptions_WithMessageBufferSizeZero_NormalizedToDefault(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(WithMessageBufferSize(0))

	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
}

func TestApplyHandlerOptions_WithMessageBufferSizeNegative_NormalizedToDefault(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(WithMessageBufferSize(-1))

	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
}

func TestApplyHandlerOptions_WithMaxDispatchTimeout_SetsField(t *testing.T) {
	t.Parallel()
	d := 50 * time.Millisecond
	cfg := applyHandlerOptions(WithMaxDispatchTimeout(d))

	assert.Equal(t, d, cfg.maxDispatchTimeout)
}

func TestApplyHandlerOptions_WithMaxDispatchTimeoutZero_KeepsZero(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(WithMaxDispatchTimeout(0))

	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
}

func TestApplyHandlerOptions_WithMessageOnly_SetsField(t *testing.T) {
	t.Parallel()

	cfgTrue := applyHandlerOptions(WithMessageOnly(true))
	assert.True(t, cfgTrue.messageOnly)

	cfgFalse := applyHandlerOptions(WithMessageOnly(false))
	assert.False(t, cfgFalse.messageOnly)
}

func TestApplyHandlerOptions_MultipleOptions_AllApplied(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(
		WithMaxPanicAttempts(5),
		WithPanicBackoff(3*time.Second),
		WithMessageBufferSize(20),
		WithMaxDispatchTimeout(100*time.Millisecond),
		WithMessageOnly(true),
	)

	assert.Equal(t, 5, cfg.maxPanicAttempts)
	assert.Equal(t, 3*time.Second, cfg.panicBackoff)
	assert.Equal(t, 20, cfg.messageBufferSize)
	assert.Equal(t, 100*time.Millisecond, cfg.maxDispatchTimeout)
	assert.True(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_LastOptionWins_WhenSameFieldSetTwice(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(
		WithMaxPanicAttempts(2),
		WithMaxPanicAttempts(7),
	)
	assert.Equal(t, 7, cfg.maxPanicAttempts)

	cfg2 := applyHandlerOptions(
		WithMessageBufferSize(5),
		WithMessageBufferSize(0), // normalized to default
	)
	assert.Equal(t, defaultMessageBufferSize, cfg2.messageBufferSize)
}

func TestApplyHandlerOptions_ZeroValuesNormalized_WhenMixedWithOtherOptions(t *testing.T) {
	t.Parallel()
	cfg := applyHandlerOptions(
		WithMaxPanicAttempts(1),
		WithMessageBufferSize(0),  // should become defaultMessageBufferSize
		WithPanicBackoff(0),       // should become defaultPanicBackoff
		WithMaxDispatchTimeout(0), // stays 0 (no timeout)
	)

	require.Equal(t, 1, cfg.maxPanicAttempts)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
}

func TestDefaultConstants_Values(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 1, defaultMessageBufferSize)
	assert.Equal(t, time.Second, defaultPanicBackoff)
}
