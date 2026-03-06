package smoothoperator_test

import (
	"errors"
	"testing"
	"time"

	"github.com/diego-miranda-ng/smoothoperator"
	"github.com/stretchr/testify/require"
)

func TestNone_ReturnsHandleResultWithStatusNoneAndIdleDuration(t *testing.T) {
	t.Parallel()

	// Arrange
	idle := 2 * time.Second

	// Act
	result := smoothoperator.None(idle)

	// Assert
	require.Equal(t, smoothoperator.HandleStatusNone, result.Status)
	require.Equal(t, idle, result.IdleDuration)
	require.Nil(t, result.Err)
	require.Nil(t, result.Result)
}

func TestNone_WithZeroDuration_ReturnsIdleZero(t *testing.T) {
	t.Parallel()

	// Act
	result := smoothoperator.None(0)

	// Assert
	require.Equal(t, smoothoperator.HandleStatusNone, result.Status)
	require.Equal(t, time.Duration(0), result.IdleDuration)
}

func TestDone_ReturnsHandleResultWithStatusDone(t *testing.T) {
	t.Parallel()

	// Act
	result := smoothoperator.Done()

	// Assert
	require.Equal(t, smoothoperator.HandleStatusDone, result.Status)
	require.Equal(t, time.Duration(0), result.IdleDuration)
	require.Nil(t, result.Err)
	require.Nil(t, result.Result)
}

func TestDoneWithResult_ReturnsHandleResultWithStatusDoneAndResult(t *testing.T) {
	t.Parallel()

	// Arrange
	payload := "payload"

	// Act
	result := smoothoperator.DoneWithResult(payload)

	// Assert
	require.Equal(t, smoothoperator.HandleStatusDone, result.Status)
	require.Equal(t, payload, result.Result)
	require.Nil(t, result.Err)
}

func TestFail_ReturnsHandleResultWithStatusFailErrAndIdleDuration(t *testing.T) {
	t.Parallel()

	// Arrange
	e := errors.New("fail")
	idle := 2 * time.Second

	// Act
	result := smoothoperator.Fail(e, idle)

	// Assert
	require.Equal(t, smoothoperator.HandleStatusFail, result.Status)
	require.Equal(t, e, result.Err)
	require.Equal(t, idle, result.IdleDuration)
	require.Nil(t, result.Result)
}

func TestFail_WithZeroIdle_KeepsZeroIdle(t *testing.T) {
	t.Parallel()

	// Arrange
	e := errors.New("fail")

	// Act
	result := smoothoperator.Fail(e, 0)

	// Assert
	require.Equal(t, smoothoperator.HandleStatusFail, result.Status)
	require.Equal(t, time.Duration(0), result.IdleDuration)
}
