package smoothoperator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyHandlerOptions_NoOptions_ReturnsDefaults(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions()

	// Assert
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize, "messageBufferSize should be default")
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff, "panicBackoff should be default")
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMaxPanicAttempts_SetsField(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(WithMaxPanicAttempts(3))

	// Assert: only maxPanicAttempts is set; all others stay default
	assert.Equal(t, 3, cfg.maxPanicAttempts)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMaxPanicAttemptsZero_KeepsZero(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(WithMaxPanicAttempts(0))

	// Assert: only maxPanicAttempts is 0; all others stay default
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithPanicBackoff_SetsField(t *testing.T) {
	t.Parallel()

	// Arrange
	d := 2 * time.Second
	// Act
	cfg := applyHandlerOptions(WithPanicBackoff(d))

	// Assert: only panicBackoff is set; all others stay default
	assert.Equal(t, d, cfg.panicBackoff)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithPanicBackoffZero_NormalizedToDefault(t *testing.T) {
	t.Parallel()
	// Arrange
	// Act
	cfg := applyHandlerOptions(WithPanicBackoff(0))

	// Assert: panicBackoff normalized to default; all others stay default
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMessageBufferSize_SetsField(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(WithMessageBufferSize(10))

	// Assert: only messageBufferSize is set; all others stay default
	assert.Equal(t, 10, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMessageBufferSizeZero_NormalizedToDefault(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(WithMessageBufferSize(0))

	// Assert
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMessageBufferSizeNegative_NormalizedToDefault(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(WithMessageBufferSize(-1))

	// Assert
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMaxDispatchTimeout_SetsField(t *testing.T) {
	t.Parallel()

	// Arrange
	d := 50 * time.Millisecond

	// Act
	cfg := applyHandlerOptions(WithMaxDispatchTimeout(d))

	// Assert: only maxDispatchTimeout is set; all others stay default
	assert.Equal(t, d, cfg.maxDispatchTimeout)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMaxDispatchTimeoutZero_KeepsZero(t *testing.T) {
	t.Parallel()

	// Arrange
	// Act
	cfg := applyHandlerOptions(WithMaxDispatchTimeout(0))

	// Assert: only maxDispatchTimeout is 0; all others stay default
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMessageOnly_SetsField(t *testing.T) {
	t.Parallel()
	// Arrange
	// Act
	cfgTrue := applyHandlerOptions(WithMessageOnly(true))
	cfgFalse := applyHandlerOptions(WithMessageOnly(false))

	// Assert: only messageOnly is set; all others stay default
	assert.True(t, cfgTrue.messageOnly)
	assert.Equal(t, defaultMessageBufferSize, cfgTrue.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfgTrue.panicBackoff)
	assert.Equal(t, 0, cfgTrue.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfgTrue.maxDispatchTimeout)

	assert.False(t, cfgFalse.messageOnly)
	assert.Equal(t, defaultMessageBufferSize, cfgFalse.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfgFalse.panicBackoff)
	assert.Equal(t, 0, cfgFalse.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfgFalse.maxDispatchTimeout)
}

func TestApplyHandlerOptions_MultipleOptions_AllApplied(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(
		WithMaxPanicAttempts(5),
		WithPanicBackoff(3*time.Second),
		WithMessageBufferSize(20),
		WithMaxDispatchTimeout(100*time.Millisecond),
		WithMessageOnly(true),
	)

	// Assert
	assert.Equal(t, 5, cfg.maxPanicAttempts)
	assert.Equal(t, 3*time.Second, cfg.panicBackoff)
	assert.Equal(t, 20, cfg.messageBufferSize)
	assert.Equal(t, 100*time.Millisecond, cfg.maxDispatchTimeout)
	assert.True(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_LastOptionWins_WhenSameFieldSetTwice(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(
		WithMaxPanicAttempts(2),
		WithMaxPanicAttempts(7),
	)
	cfg2 := applyHandlerOptions(
		WithMessageBufferSize(5),
		WithMessageBufferSize(0), // normalized to default
	)

	// Assert
	assert.Equal(t, 7, cfg.maxPanicAttempts)
	assert.Equal(t, defaultMessageBufferSize, cfg2.messageBufferSize)
}

func TestApplyHandlerOptions_ZeroValuesNormalized_WhenMixedWithOtherOptions(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(
		WithMaxPanicAttempts(1),
		WithMessageBufferSize(0),  // should become defaultMessageBufferSize
		WithPanicBackoff(0),       // should become defaultPanicBackoff
		WithMaxDispatchTimeout(0), // stays 0 (no timeout)
	)

	// Assert
	require.Equal(t, 1, cfg.maxPanicAttempts)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
}
