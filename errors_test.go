package smoothoperator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrWorkerAlreadyExists_IsSentinelAndContainsMessage(t *testing.T) {
	t.Parallel()
	// Arrange
	// Act
	// Assert
	require.NotNil(t, ErrWorkerAlreadyExists)
	require.Contains(t, ErrWorkerAlreadyExists.Error(), "already exists")
	require.True(t, errors.Is(ErrWorkerAlreadyExists, ErrWorkerAlreadyExists))
}

func TestErrWorkerNotFound_IsSentinelAndContainsMessage(t *testing.T) {
	t.Parallel()
	// Arrange
	// Act
	// Assert
	require.NotNil(t, ErrWorkerNotFound)
	require.Contains(t, ErrWorkerNotFound.Error(), "not found")
	require.True(t, errors.Is(ErrWorkerNotFound, ErrWorkerNotFound))
}

func TestErrDispatchTimeout_IsSentinelAndContainsMessage(t *testing.T) {
	t.Parallel()
	// Arrange
	// Act
	// Assert
	require.NotNil(t, ErrDispatchTimeout)
	require.Contains(t, ErrDispatchTimeout.Error(), "timeout")
	require.True(t, errors.Is(ErrDispatchTimeout, ErrDispatchTimeout))
}

func TestErrors_WhenWrapped_CanBeIdentifiedWithErrorsIs(t *testing.T) {
	t.Parallel()
	// Arrange: simulate operator returning wrapped error (e.g. fmt.Errorf("...: %w", ErrWorkerNotFound))
	wrapped := errors.Join(errors.New("worker foo not found"), ErrWorkerNotFound)

	// Act
	ok := errors.Is(wrapped, ErrWorkerNotFound)

	// Assert
	require.True(t, ok)
}
