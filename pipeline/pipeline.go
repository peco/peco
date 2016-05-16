package pipeline

import (
	"fmt"

	"github.com/pkg/errors"

	"golang.org/x/net/context"
)

type EndMark struct {
}

func (e EndMark) EndMark() bool {
	return true
}

func (e EndMark) Error() string {
	return "end of input"
}

type EndMarker interface {
	EndMark() bool
}

func IsEndMark(err error) bool {
	if em, ok := errors.Cause(err).(EndMarker); ok {
		fmt.Printf("is end marker!\n")
		return em.EndMark()
	}
	return false
}

type Source interface {
	Producer
	Start(context.Context)
}

type Destination interface {
	Reset()
	Done() <-chan struct{}
	Acceptor
}

type Pipeline struct {
	nodes []ProcNode
	src   Source
	dst   Destination
}

type OutputChannel chan interface{}

func (oc OutputChannel) OutCh() chan interface{} {
	return oc
}

func (oc OutputChannel) Send(v interface{}) {
	fmt.Printf("Send: '%v'\n", v)
	oc <- v
}

func (oc OutputChannel) SendEndMark(s string) {
	oc.Send(errors.Wrap(EndMark{}, s))
}

type Producer interface {
	OutCh() chan interface{}
}

type Acceptor interface {
	Accept(context.Context, Producer)
}

type ProcNode interface {
	Producer
	Acceptor
}

func New() *Pipeline {
	return &Pipeline{}
}

func (p *Pipeline) Source(s Source) {
	p.src = s
}

// Add adds new ProcNodes that work on data that goes through the Pipeline.
func (p *Pipeline) Add(n ProcNode) {
	p.nodes = append(p.nodes, n)
}

func (p *Pipeline) Destination(d Destination) {
	p.dst = d
}

func (p *Pipeline) Run(ctx context.Context) error {
	if p.src == nil {
		return errors.New("source must be non-nil")
	}

	if p.dst == nil {
		return errors.New("destination must be non-nil")
	}

	// Reset is called on the destination to effectively reset
	// any state changes that may have happened in the end of
	// the previous call to Run()
	p.dst.Reset()

	// Setup the ProcNodes, effectively chaining all nodes
	// starting from the destination, working all the way
	// up to the Source
	var prev Acceptor = p.dst
	for i := len(p.nodes) - 1; i >= 0; i-- {
		cur := p.nodes[i]
		go prev.Accept(ctx, cur)
		prev = cur
	}

	// Chain to Source...
	go prev.Accept(ctx, p.src)

	// And now tell the Source to send the values so data chugs
	// through the pipeline
	go p.src.Start(ctx)

	// Wait till we're done
	<-p.dst.Done()

	return nil
}
