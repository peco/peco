package peco

import (
	"errors"
	"runtime"
)

// ErrBufferOutOfRange is returned when the index within the buffer that
// was queried was out of the containing buffer's range
var ErrBufferOutOfRange = errors.New("error: Specified index is out of range")

type Pipeliner interface {
	Pipeline() (chan struct{}, chan Line)
}

type pipelineCtx struct {
	onIncomingLine func(Line) (Line, error)
	onEnd          func()
}

type simplePipeline struct {
	// Close this channel if you want to cancel the entire pipeline.
	// InputReader is the generator for the pipeline, so this is the
	// only object that can create the cancelCh
	cancelCh chan struct{}
	// Consumers of this generator read from this channel.
	outputCh chan Line
}

func (sp simplePipeline) Cancel()                 { close(sp.cancelCh) }
func (sp simplePipeline) CancelCh() chan struct{} { return sp.cancelCh }
func (sp simplePipeline) OutputCh() chan Line     { return sp.outputCh }
func (sp simplePipeline) Pipeline() (chan struct{}, chan Line) {
	return sp.cancelCh, sp.outputCh
}

func acceptPipeline(cancel chan struct{}, in chan Line, out chan Line, pc *pipelineCtx) {
	trace("acceptPipeline: START")
	defer trace("acceptPipeline: END")
	defer close(out)
	for {
		select {
		case <-cancel:
			trace("acceptPipeline: detected cancel request. Bailing out")
			return
		case l, ok := <-in:
			if l == nil && !ok {
				trace("acceptPipeline: detected end of input. Bailing out")
				if pc.onEnd != nil {
					pc.onEnd()
				}
				return
			}
			trace("acceptPipeline: forwarding to callback")
			if ll, err := pc.onIncomingLine(l); err == nil {
				trace("acceptPipeline: forwarding to out channel")
				out <- ll
			}
		}
	}
}

// LineBuffer represents a set of lines. This could be the
// raw data read in, or filtered data, such as result of
// running a match, or applying a selection by the user
//
// Buffers should be immutable.
type LineBuffer interface {
	Pipeliner

	LineAt(int) (Line, error)
	Size() int

	// Register registers another LineBuffer that is dependent on
	// this buffer.
	Register(LineBuffer)
	Unregister(LineBuffer)

	// InvalidateUpTo is called when a source buffer invalidates
	// some lines. The argument is the largest line number that
	// should be invalidated (so anything up to that line is no
	// longer valid in the source)
	InvalidateUpTo(int)
}

type dependentBuffers []LineBuffer

func (buffers *dependentBuffers) Register(lb LineBuffer) {
	*buffers = append(*buffers, lb)
}

func (buffers *dependentBuffers) Unregister(lb LineBuffer) {
	for i, x := range *buffers {
		if x == lb {
			switch i {
			case 0:
				*buffers = append([]LineBuffer(nil), (*buffers)[1:]...)
			case len(*buffers) - 1:
				*buffers = append([]LineBuffer(nil), (*buffers)[0:i-1]...)
			default:
				*buffers = append(append([]LineBuffer(nil), (*buffers)[0:i-1]...), (*buffers)[i+1:]...)
			}
			return
		}
	}
}

func (buffers dependentBuffers) InvalidateUpTo(i int) {
	for _, b := range buffers {
		b.InvalidateUpTo(i)
	}
}

// RawLineBuffer holds the raw set of lines as read into peco.
type RawLineBuffer struct {
	simplePipeline
	buffers  dependentBuffers
	lines    []Line
	capacity int // max numer of lines. 0 means unlimited
	onEnd    func()
}

func NewRawLineBuffer() *RawLineBuffer {
	return &RawLineBuffer{
		simplePipeline: simplePipeline{},
		lines:          []Line{},
		capacity:       0,
	}
}

func (rlb *RawLineBuffer) Replay() error {
	rlb.outputCh = make(chan Line)
	go func() {
		replayed := 0
		trace("RawLineBuffer.Replay (goroutine): START")
		defer func() { trace("RawLineBuffer.Replay (goroutine): END (Replayed %d lines)", replayed) }()

		defer func() { recover() }() // It's okay if we fail to replay
		defer close(rlb.outputCh)
		for _, l := range rlb.lines {
			rlb.outputCh <- l
			replayed++
		}
	}()
	return nil
}

func (rlb *RawLineBuffer) Accept(p Pipeliner) {
	cancelCh, incomingCh := p.Pipeline()
	rlb.cancelCh = cancelCh
	rlb.outputCh = make(chan Line)
	go acceptPipeline(cancelCh, incomingCh, rlb.outputCh,
		&pipelineCtx{rlb.Append, rlb.onEnd})
}

func (rlb *RawLineBuffer) Append(l Line) (Line, error) {
	trace("RawLineBuffer.Append: %s", l.DisplayString())
	if rlb.capacity > 0 && len(rlb.lines) > rlb.capacity {
		diff := len(rlb.lines) - rlb.capacity

		// Golang's version of array realloc
		rlb.lines = rlb.lines[diff:rlb.capacity:rlb.capacity]
	} else {
		rlb.lines = append(rlb.lines, l)
	}

	return l, nil
}

func (rlb *RawLineBuffer) Register(lb LineBuffer) {
	rlb.buffers.Register(lb)
}

func (rlb *RawLineBuffer) Unregister(lb LineBuffer) {
	rlb.buffers.Unregister(lb)
}

// LineAt returns the line at index `i`
func (rlb RawLineBuffer) LineAt(i int) (Line, error) {
	if i < 0 || len(rlb.lines) <= i {
		return nil, ErrBufferOutOfRange
	}
	return rlb.lines[i], nil
}

// Size returns the number of lines in the buffer
func (rlb RawLineBuffer) Size() int {
	return len(rlb.lines)
}

func (rlb *RawLineBuffer) SetCapacity(capacity int) {
	if capacity < 0 {
		capacity = 0
	}
	rlb.capacity = capacity
}

func (rlb RawLineBuffer) InvalidateUpTo(_ int) {
	// no op
}

func (rlb *RawLineBuffer) AppendLine(l Line) (Line, error) {
	return rlb.Append(l)
}

// FilteredLineBuffer holds a "filtered" buffer. It holds a reference to
// the source buffer (note: should be immutable) and a list of indices
// into the source buffer
type FilteredLineBuffer struct {
	simplePipeline
	buffers dependentBuffers
	src     LineBuffer
	// maps from our index to src's index
	selection []int
}

func NewFilteredLineBuffer(src LineBuffer) *FilteredLineBuffer {
	flb := &FilteredLineBuffer{
		simplePipeline: simplePipeline{},
		src:            src,
		selection:      []int{},
	}
	src.Register(flb)

	runtime.SetFinalizer(flb, func(x *FilteredLineBuffer) {
		x.src.Unregister(x)
	})

	return flb
}

func (flb *FilteredLineBuffer) Accept(p Pipeliner) {
	cancelCh, incomingCh := p.Pipeline()
	flb.cancelCh = cancelCh
	flb.outputCh = make(chan Line)
	go acceptPipeline(cancelCh, incomingCh, flb.outputCh,
		&pipelineCtx{flb.Append, nil})
}

func (flb *FilteredLineBuffer) Append(l Line) (Line, error) {
	return l, nil
}

func (flb *FilteredLineBuffer) InvalidateUpTo(x int) {
	p := -1
	for i := 0; i < len(flb.selection); i++ {
		if flb.selection[i] > x {
			break
		}
		p = i
	}

	if p >= 0 {
		flb.selection = append([]int(nil), flb.selection[p:]...)
	}

	for _, b := range flb.buffers {
		b.InvalidateUpTo(p)
	}
}

// LineAt returns the line at index `i`. Note that the i-th element
// in this filtered buffer may actually correspond to a totally
// different line number in the source buffer.
func (flb FilteredLineBuffer) LineAt(i int) (Line, error) {
	if i < 0 || i >= len(flb.selection) {
		return nil, ErrBufferOutOfRange
	}
	return flb.src.LineAt(flb.selection[i])
}

// Size returns the number of lines in the buffer
func (flb FilteredLineBuffer) Size() int {
	return len(flb.selection)
}

func (flb *FilteredLineBuffer) SelectSourceLineAt(i int) {
	flb.selection = append(flb.selection, i)
}

func (flb *FilteredLineBuffer) Register(lb LineBuffer) {
	flb.buffers.Register(lb)
}

func (flb *FilteredLineBuffer) Unregister(lb LineBuffer) {
	flb.buffers.Unregister(lb)
}

