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
	cfg := applyHandlerOptions(config{})

	// Assert
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize, "messageBufferSize should be default")
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff, "panicBackoff should be default")
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
	assert.False(t, cfg.lockOSThread)
}

func TestApplyHandlerOptions_WithMaxPanicAttempts_SetsField(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(config{}, WithMaxPanicAttempts(3))

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
	cfg := applyHandlerOptions(config{}, WithMaxPanicAttempts(0))

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
	cfg := applyHandlerOptions(config{}, WithPanicBackoff(d))

	// Assert: only panicBackoff is set; all others stay default
	assert.Equal(t, d, cfg.panicBackoff)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithPanicBackoffZero_NormalizedToDefault(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(config{}, WithPanicBackoff(0))

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
	cfg := applyHandlerOptions(config{}, WithMessageBufferSize(10))

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
	cfg := applyHandlerOptions(config{}, WithMessageBufferSize(0))

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
	cfg := applyHandlerOptions(config{}, WithMessageBufferSize(-1))

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
	cfg := applyHandlerOptions(config{}, WithMaxDispatchTimeout(d))

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
	cfg := applyHandlerOptions(config{}, WithMaxDispatchTimeout(0))

	// Assert: only maxDispatchTimeout is 0; all others stay default
	assert.Equal(t, time.Duration(0), cfg.maxDispatchTimeout)
	assert.Equal(t, defaultMessageBufferSize, cfg.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfg.panicBackoff)
	assert.Equal(t, 0, cfg.maxPanicAttempts)
	assert.False(t, cfg.messageOnly)
}

func TestApplyHandlerOptions_WithMessageOnly_SetsField(t *testing.T) {
	t.Parallel()

	// Act
	cfgTrue := applyHandlerOptions(config{}, WithMessageOnly(true))
	cfgFalse := applyHandlerOptions(config{}, WithMessageOnly(false))

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
	cfg := applyHandlerOptions(config{},
		WithMaxPanicAttempts(5),
		WithPanicBackoff(3*time.Second),
		WithMessageBufferSize(20),
		WithMaxDispatchTimeout(100*time.Millisecond),
		WithMessageOnly(true),
		WithLockOSThread(true),
	)

	// Assert
	assert.Equal(t, 5, cfg.maxPanicAttempts)
	assert.Equal(t, 3*time.Second, cfg.panicBackoff)
	assert.Equal(t, 20, cfg.messageBufferSize)
	assert.Equal(t, 100*time.Millisecond, cfg.maxDispatchTimeout)
	assert.True(t, cfg.messageOnly)
	assert.True(t, cfg.lockOSThread)
}

func TestApplyHandlerOptions_LastOptionWins_WhenSameFieldSetTwice(t *testing.T) {
	t.Parallel()

	// Act
	cfg := applyHandlerOptions(config{},
		WithMaxPanicAttempts(2),
		WithMaxPanicAttempts(7),
	)
	cfg2 := applyHandlerOptions(config{},
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
	cfg := applyHandlerOptions(config{},
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

func TestApplyHandlerOptions_WithLockOSThread_SetsField(t *testing.T) {
	t.Parallel()

	// Act
	cfgTrue := applyHandlerOptions(config{}, WithLockOSThread(true))
	cfgFalse := applyHandlerOptions(config{}, WithLockOSThread(false))

	// Assert: only lockOSThread is set; all others stay default
	assert.True(t, cfgTrue.lockOSThread)
	assert.Equal(t, defaultMessageBufferSize, cfgTrue.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfgTrue.panicBackoff)
	assert.Equal(t, 0, cfgTrue.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfgTrue.maxDispatchTimeout)
	assert.False(t, cfgTrue.messageOnly)

	assert.False(t, cfgFalse.lockOSThread)
	assert.Equal(t, defaultMessageBufferSize, cfgFalse.messageBufferSize)
	assert.Equal(t, defaultPanicBackoff, cfgFalse.panicBackoff)
	assert.Equal(t, 0, cfgFalse.maxPanicAttempts)
	assert.Equal(t, time.Duration(0), cfgFalse.maxDispatchTimeout)
	assert.False(t, cfgFalse.messageOnly)
}

func TestApplyHandlerOptions_WithExistingBase_ShouldPreserveUnchangedFields(t *testing.T) {
	t.Parallel()

	// Arrange: base config with non-default values
	base := config{
		maxPanicAttempts:   5,
		panicBackoff:       3 * time.Second,
		messageBufferSize:  20,
		maxDispatchTimeout: 100 * time.Millisecond,
		messageOnly:        true,
		lockOSThread:       false,
	}

	// Act: only change lockOSThread; everything else should be preserved
	cfg := applyHandlerOptions(base, WithLockOSThread(true))

	// Assert
	assert.Equal(t, 5, cfg.maxPanicAttempts)
	assert.Equal(t, 3*time.Second, cfg.panicBackoff)
	assert.Equal(t, 20, cfg.messageBufferSize)
	assert.Equal(t, 100*time.Millisecond, cfg.maxDispatchTimeout)
	assert.True(t, cfg.messageOnly)
	assert.True(t, cfg.lockOSThread)
}

func TestDefaultConstants_Values(t *testing.T) {
	t.Parallel()

	// Assert
	assert.Equal(t, 1, defaultMessageBufferSize)
	assert.Equal(t, time.Second, defaultPanicBackoff)
}
