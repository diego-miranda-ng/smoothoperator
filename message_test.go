package smoothoperator_test

import (
	"context"
	"testing"
	"time"

	"github.com/diego-miranda-ng/smoothoperator"
	"github.com/diego-miranda-ng/smoothoperator/internal"
	"github.com/stretchr/testify/require"
)

func TestSendMessage_WhenWorkerExists_DeliversTypedMessage(t *testing.T) {
	t.Parallel()
	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	recorder := newMessageRecorder(5 * time.Second)
	_, err := op.AddHandler("w", recorder.Handler())
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(30 * time.Millisecond)

	// Act
	delivered, resultCh, err := smoothoperator.SendMessage[string](op, "w", "hello")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, delivered)
	require.NotNil(t, resultCh)
	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message not delivered")
	}
	<-resultCh
	<-op.StopAll()
	msgs := recorder.Messages()
	require.Len(t, msgs, 1)
	m, ok := msgs[0].(smoothoperator.Message[string])
	require.True(t, ok)
	require.Equal(t, "hello", m.Data)
}

func TestSendMessage_WhenWorkerNotFound_ReturnsError(t *testing.T) {
	t.Parallel()
	// Arrange
	op := smoothoperator.NewOperator(context.Background())

	// Act
	delivered, resultCh, err := smoothoperator.SendMessage[string](op, "missing", "data")

	// Assert
	require.Error(t, err)
	require.Nil(t, delivered)
	require.Nil(t, resultCh)
	require.True(t, err != nil && (err.Error() == "worker not found" || len(err.Error()) > 0))
}

func TestSendMessageWithContext_WhenWorkerExists_DeliversMessage(t *testing.T) {
	t.Parallel()
	// Arrange
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", internal.NewHandlerMock(func(ctx context.Context, msg any) smoothoperator.HandleResult {
		return smoothoperator.Done()
	}))
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(30 * time.Millisecond)
	ctx := context.Background()

	// Act
	delivered, resultCh, err := smoothoperator.SendMessageWithContext(ctx, op, "w", "with-ctx")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, delivered)
	require.NotNil(t, resultCh)
	select {
	case <-delivered:
	case <-time.After(2 * time.Second):
		t.Fatal("message not delivered")
	}
	<-resultCh
	<-op.StopAll()
}

func TestSendMessageWithContext_WhenContextTimesOut_ReturnsError(t *testing.T) {
	t.Parallel()
	// Arrange: worker with small buffer and blocking handler so buffer fills and next send can timeout
	op := smoothoperator.NewOperator(context.Background())
	_, err := op.AddHandler("w", blockingHandler(2*time.Second), smoothoperator.WithMessageBufferSize(1))
	require.NoError(t, err)
	require.NoError(t, op.Start("w"))
	time.Sleep(50 * time.Millisecond)
	// First message fills the buffer and is being processed (blocking)
	_, _, _ = op.Dispatch(context.Background(), "w", "first")
	_, _, _ = op.Dispatch(context.Background(), "w", "second")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	// Act
	delivered, resultCh, err := smoothoperator.SendMessageWithContext(ctx, op, "w", "third")

	// Assert
	require.Error(t, err)
	require.Nil(t, delivered)
	require.Nil(t, resultCh)
	<-op.StopAll()
}
