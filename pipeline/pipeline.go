// Package pipeline implements the basic data processing pipeline used by peco
package pipeline

import (
	"context"
	"errors"

	pdebug "github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/line"
)

type queryContextKey struct{}

// NewQueryContext returns a context with the query string stored under a typed key.
func NewQueryContext(ctx context.Context, query string) context.Context {
	return context.WithValue(ctx, queryContextKey{}, query)
}

// QueryFromContext retrieves the query string from the context.
func QueryFromContext(ctx context.Context) string {
	v, _ := ctx.Value(queryContextKey{}).(string)
	return v
}

func NilOutput(ctx context.Context) ChanOutput {
	ch := make(chan line.Line)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
			}
		}
	}()

	return ChanOutput(ch)
}

// OutCh returns the channel that acceptors can listen to
func (oc ChanOutput) OutCh() <-chan line.Line {
	return oc
}

// Send sends the data `v` through this channel. It blocks until the value
// is sent or the context is cancelled. This avoids the timer allocation
// overhead of the previous implementation while still supporting cancellation.
func (oc ChanOutput) Send(ctx context.Context, v line.Line) (err error) {
	if oc == nil {
		return errors.New("nil channel")
	}

	select {
	case oc <- v:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// New creates a new Pipeline
func New() *Pipeline {
	return &Pipeline{
		done: make(chan struct{}),
	}
}

// SetSource sets the source.
// If called during `Run`, this method will block.
func (p *Pipeline) SetSource(s Source) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.src = s
}

// Add adds new Acceptor that work on data that goes through the Pipeline.
// If called during `Run`, this method will block.
func (p *Pipeline) Add(n Acceptor) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.nodes = append(p.nodes, n)
}

// SetDestination sets the destination.
// If called during `Run`, this method will block.
func (p *Pipeline) SetDestination(d Destination) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.dst = d
}

// Run starts the processing. Mutator methods for `Pipeline` cannot be
// called while `Run` is running.
func (p *Pipeline) Run(ctx context.Context) (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Pipeline.Run (%s)", QueryFromContext(ctx)).BindError(&err)
		defer g.End()
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	defer close(p.done)

	if p.src == nil {
		return errors.New("source must be non-nil")
	}

	if p.dst == nil {
		return errors.New("destination must be non-nil")
	}

	// Reset is called on the source/destination to effectively reset
	// any state changes that may have happened in the end of
	// the previous call to Run()
	p.src.Reset()
	p.dst.Reset()

	// Setup the Acceptors, effectively chaining all nodes
	// starting from the destination, working all the way
	// up to the Source
	// Use buffered channels between pipeline stages to allow pipelining
	const chanBufSize = 256

	prevCh := ChanOutput(make(chan line.Line, chanBufSize))
	go p.dst.Accept(ctx, prevCh, nil)

	for i := len(p.nodes) - 1; i >= 0; i-- {
		cur := p.nodes[i]
		ch := make(chan line.Line, chanBufSize)
		go cur.Accept(ctx, ch, prevCh)
		prevCh = ChanOutput(ch)
	}

	// And now tell the Source to send the values so data chugs
	// through the pipeline
	go p.src.Start(ctx, prevCh)

	// Wait till we're done
	<-p.dst.Done()

	return nil
}

func (p *Pipeline) Done() <-chan struct{} {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.done
}
