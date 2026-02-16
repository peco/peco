package pipeline

import (
	"sync"

	"context"

	"github.com/peco/peco/line"
)

type Source interface {
	// Start should be able to be called repeatedly, producing the
	// same data to be consumed by the chained Acceptors.
	// The implementation must close out when done sending.
	Start(context.Context, ChanOutput)

	Reset()
}

type Suspender interface {
	Suspend()
	Resume()
}

// Acceptor is an object that can accept input, and send to
// an optional output. The implementation must close out (if non-nil)
// when in is exhausted or the context is cancelled.
type Acceptor interface {
	Accept(context.Context, <-chan line.Line, ChanOutput)
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

// ChanOutput is a typed channel for sending line.Line values between
// pipeline stages.
type ChanOutput chan line.Line
