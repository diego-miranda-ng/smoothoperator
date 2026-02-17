package smoothoperator

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNone_ReturnsHandleResultWithStatusNoneAndIdleDuration(t *testing.T) {
	t.Parallel()
	// Arrange
	idle := 2 * time.Second

	// Act
	result := None(idle)

	// Assert
	require.Equal(t, HandleStatusNone, result.Status)
	require.Equal(t, idle, result.IdleDuration)
	require.Nil(t, result.Err)
	require.Nil(t, result.Result)
}

func TestNone_WithZeroDuration_ReturnsIdleZero(t *testing.T) {
	t.Parallel()
	// Arrange
	// Act
	result := None(0)

	// Assert
	require.Equal(t, HandleStatusNone, result.Status)
	require.Equal(t, time.Duration(0), result.IdleDuration)
}

func TestDone_ReturnsHandleResultWithStatusDone(t *testing.T) {
	t.Parallel()
	// Arrange
	// Act
	result := Done()

	// Assert
	require.Equal(t, HandleStatusDone, result.Status)
	require.Equal(t, time.Duration(0), result.IdleDuration)
	require.Nil(t, result.Err)
	require.Nil(t, result.Result)
}

func TestDoneWithResult_ReturnsHandleResultWithStatusDoneAndResult(t *testing.T) {
	t.Parallel()
	// Arrange
	payload := "payload"

	// Act
	result := DoneWithResult(payload)

	// Assert
	require.Equal(t, HandleStatusDone, result.Status)
	require.Equal(t, payload, result.Result)
	require.Nil(t, result.Err)
}

func TestFail_ReturnsHandleResultWithStatusFailErrAndIdleDuration(t *testing.T) {
	t.Parallel()
	// Arrange
	e := errors.New("fail")
	idle := 2 * time.Second

	// Act
	result := Fail(e, idle)

	// Assert
	require.Equal(t, HandleStatusFail, result.Status)
	require.Equal(t, e, result.Err)
	require.Equal(t, idle, result.IdleDuration)
	require.Nil(t, result.Result)
}

func TestFail_WithZeroIdle_KeepsZeroIdle(t *testing.T) {
	t.Parallel()
	// Arrange
	e := errors.New("fail")

	// Act
	result := Fail(e, 0)

	// Assert
	require.Equal(t, HandleStatusFail, result.Status)
	require.Equal(t, time.Duration(0), result.IdleDuration)
}
