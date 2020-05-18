// Package pipeline implements the basic data processing pipeline used by peco
package pipeline

import (
	"time"

	"context"

	pdebug "github.com/lestrrat-go/pdebug/v2"
	"github.com/pkg/errors"
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
	if em, ok := errors.Cause(err).(EndMarker); ok {
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

// Send sends the data `v` through this channel
func (oc ChanOutput) Send(v interface{}) (err error) {
	if oc == nil {
		return errors.New("nil channel")
	}

	// We allow ourselves a timeout of 1 second.
	t := time.NewTimer(time.Second)
	defer t.Stop()

	select {
	case oc <- v:
	case <-t.C:
		return errors.New("failed to send (not listening)")
	}
	return nil
}

// SendEndMark sends an end mark
func (oc ChanOutput) SendEndMark(s string) error {
	return errors.Wrap(oc.Send(errors.Wrap(EndMark{}, s)), "failed to send end mark")
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
		g := pdebug.Marker(ctx, "Pipeline.Run (%s)", ctx.Value("query")).BindError(&err)
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
	var prevCh ChanOutput = ChanOutput(make(chan interface{}))
	go p.dst.Accept(ctx, prevCh, nil)

	for i := len(p.nodes) - 1; i >= 0; i-- {
		cur := p.nodes[i]
		ch := make(chan interface{}) //
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
