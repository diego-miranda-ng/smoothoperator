package smoothoperator

// Message is a generic wrapper for typed messages sent to a worker via SendMessage.
// The handler receives a Message as the msg parameter (typed as any) and can
// type-assert it to Message[T] to access the typed Data field.
type Message[T any] struct {
	// Data is the payload of the message.
	Data T
}

// envelope is an internal wrapper that pairs a message with a delivery signal.
// The worker closes delivered after unwrapping the envelope and passing msg to
// the handler's Handle method.
type envelope struct {
	msg       any
	delivered chan struct{}
}

// SendMessage sends a typed message to the worker registered under the given name.
// The data is wrapped in a Message[T] and delivered to the worker's handler via
// the msg parameter of Handle. If the worker is idle, it wakes up immediately.
// Returns a channel that is closed once the handler has received the message,
// and an error if the worker is not found.
func SendMessage[T any](op Operator, name string, data T) (<-chan struct{}, error) {
	return op.Send(name, Message[T]{Data: data})
}
