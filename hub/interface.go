package hub

import "sync"

// Hub acts as the messaging hub between components -- that is,
// it controls how the communication that goes through channels
// are handled.
type Hub struct {
	isSync      bool
	mutex       sync.Mutex
	queryCh     chan Payload
	drawCh      chan Payload
	statusMsgCh chan Payload
	pagingCh    chan Payload
}

// Payload is a wrapper around the actual request value that needs
// to be passed. It contains an optional channel field which can
// be filled to force synchronous communication between the
// sender and receiver
type Payload interface {
	// Data allows you to retrieve the data that's embedded in the payload.
	Data() interface{}

	// Done is called when the payload has been processed. There are cases
	// where the payload processing is expected to be synchronized with the
	// caller, and this method signals the caller that it is safe to proceed
	// to whatever next action it was expecting
	Done()
}

type payload struct {
	data interface{}
	done chan struct{}
}
