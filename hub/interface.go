package hub

import "sync"

// Hub acts as the messaging hub between components -- that is,
// it controls how the communication that goes through channels
// are handled.
type Hub struct {
	isSync      bool
	mutex       sync.Mutex
	queryCh     chan *Payload[string]
	drawCh      chan *Payload[*DrawOptions]
	statusMsgCh chan *Payload[StatusMsg]
	pagingCh    chan *Payload[PagingRequest]
}

// Payload is a wrapper around the actual request value that needs
// to be passed. It contains an optional channel field which can
// be filled to force synchronous communication between the
// sender and receiver
type Payload[T any] struct {
	batch bool
	data  T
	done  chan struct{}
}
