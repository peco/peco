// Package pipeline implements the basic data processing pipeline used by peco
package pipeline

import (
	"context"
	"errors"
	"fmt"

	pdebug "github.com/lestrrat-go/pdebug"
)

// EndMark returns true
func (e EndMark) EndMark() bool {
	return true
}

// Error returns the error string "end of input"
func (e EndMark) Error() string {
	return "end of input"
}

// IsEndMark is an utility function that checks if the given error
// object is an EndMark
func IsEndMark(err error) bool {
	var em EndMarker
	if errors.As(err, &em) {
		return em.EndMark()
	}
	return false
}

func NilOutput(ctx context.Context) ChanOutput {
	ch := make(chan interface{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
			}
		}
	}()

	return ChanOutput(ch)
}

// OutCh returns the channel that acceptors can listen to
func (oc ChanOutput) OutCh() <-chan interface{} {
	return oc
}

// Send sends the data `v` through this channel. It blocks until the value
// is sent or the context is cancelled. This avoids the timer allocation
// overhead of the previous implementation while still supporting cancellation.
func (oc ChanOutput) Send(ctx context.Context, v interface{}) (err error) {
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

// SendEndMark sends an end mark. If ctx is cancelled, the end mark is
// dropped since all pipeline stages are shutting down via context anyway.
func (oc ChanOutput) SendEndMark(ctx context.Context, s string) error {
	if err := oc.Send(ctx, fmt.Errorf("%s: %w", s, EndMark{})); err != nil {
		return fmt.Errorf("failed to send end mark: %w", err)
	}
	return nil
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
		g := pdebug.Marker("Pipeline.Run (%s)", ctx.Value("query")).BindError(&err)
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

	prevCh := ChanOutput(make(chan interface{}, chanBufSize))
	go p.dst.Accept(ctx, prevCh, nil)

	for i := len(p.nodes) - 1; i >= 0; i-- {
		cur := p.nodes[i]
		ch := make(chan interface{}, chanBufSize)
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
