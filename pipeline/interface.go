package pipeline

import (
	"sync"

	"golang.org/x/net/context"
)

// EndMarker is an interface for things that tell us the input
// sequence has ended
type EndMarker interface {
	error
	EndMark() bool
}

// EndMark is a dummy struct that gets send as an EOL mark of sorts
type EndMark struct{}

// Producer is an object that can generated output to be consumed
// by reading from `OutCh`
type Producer interface {
	OutCh() <-chan interface{}
}

// Source is a special type of Producer that has no previous Producers
// chained to feed it more data.
type Source interface {
	Producer

	// Start should be able to be called repeatedly, producing the
	// same data to be consumed by the chained Acceptors
	Start(context.Context)

	Reset()
}

// Acceptor is an object that can accept output from Producers
type Acceptor interface {
	Accept(context.Context, Producer)
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
	nodes []ProcNode
	src   Source
	dst   Destination
}

// OutputChannel is an alias to `chan interface{}`
type OutputChannel chan interface{}

// ProcNode is an interface that goes in between `Source` and `Destination`
type ProcNode interface {
	Producer
	Acceptor
}
