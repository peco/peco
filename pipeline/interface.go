package pipeline

import (
	"sync"

	"context"
)

// EndMarker is an interface for things that tell us the input
// sequence has ended
type EndMarker interface {
	error
	EndMark() bool
}

// EndMark is a dummy struct that gets send as an EOL mark of sorts
type EndMark struct{}

type Source interface {
	// Start should be able to be called repeatedly, producing the
	// same data to be consumed by the chained Acceptors
	Start(context.Context, ChanOutput)

	Reset()
}

// Acceptor is an object that can accept input, and send to
// an optional output
type Acceptor interface {
	Accept(context.Context, chan interface{}, ChanOutput)
}

// Destination is a special case Acceptor that has no more Acceptors
// chained to it to consume data
type Destination interface {
	Reset()
	Done() <-chan struct{}
	Acceptor
}

// Pipeline is encapsulates a chain of `Source`, `ProcNode`s, and `Destination`
type Pipeline struct {
	done  chan struct{}
	mutex sync.Mutex
	nodes []Acceptor
	src   Source
	dst   Destination
}

type Output interface {
	Send(interface{}) error
}

// ChanOutput is an alias to `chan interface{}`
type ChanOutput chan interface{}
