package smoothoperator

import "context"

// Message is a generic wrapper for typed messages sent to a worker via SendMessage.
// The handler receives a Message as the msg parameter (typed as any) and can
// type-assert it to Message[T] to access the typed Data field.
type Message[T any] struct {
	// Data is the payload of the message.
	Data T
}

// envelope is an internal wrapper that pairs a message with delivery and result signals.
// The worker closes delivered after passing msg to the handler, then sends
// HandleResult.Result on resultCh (if present) and closes resultCh when the handler returns.
type envelope struct {
	msg       any
	delivered chan struct{}
	resultCh  chan any // buffered 1; worker sends Result then closes
}

// SendMessage sends a typed message to the worker registered under the given name,
// using context.Background() so the send blocks until the worker has buffer space.
// For timeout or cancellation use SendMessageWithContext. The data is wrapped in a
// Message[T] and delivered to the worker's handler via the msg parameter of Handle.
// Returns: a channel that closes once the handler has received the message; a
// channel that receives the handler's Result (from HandleResult) when the handler
// finishes, then closes; and an error if the worker is not found.
func SendMessage[T any](op Operator, name string, data T) (<-chan struct{}, <-chan any, error) {
	return op.Dispatch(context.Background(), name, Message[T]{Data: data})
}

// SendMessageWithContext sends a typed message to the worker using the given context.
// If the worker's message buffer is full, Dispatch blocks until there is space or
// ctx is done (e.g. timeout); on ctx.Done it returns ctx.Err() without sending.
func SendMessageWithContext[T any](ctx context.Context, op Operator, name string, data T) (<-chan struct{}, <-chan any, error) {
	return op.Dispatch(ctx, name, Message[T]{Data: data})
}
