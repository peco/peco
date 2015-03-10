package peco

import (
	"errors"
	"runtime"
)

var ErrBufferOutOfRange = errors.New("error: Specified index is out of range")

type Pipeliner interface {
	Pipeline() (chan struct{}, chan Line)
}

type PipelineComponent struct {
	onIncomingLine func(Line) (Line, error)
	onEnd func()
}

type simplePipeline struct {
	// Close this channel if you want to cancel the entire pipeline.
	// InputReader is the generator for the pipeline, so this is the
	// only object that can create the cancelCh
	cancelCh chan struct{}
	// Consumers of this generator read from this channel.
	outputCh chan Line
}

func (sp simplePipeline) Cancel() { close(sp.cancelCh) }
func (sp simplePipeline) CancelCh() chan struct{} { return sp.cancelCh }
func (sp simplePipeline) OutputCh() chan Line     { return sp.outputCh }
func (sp simplePipeline) Pipeline() (chan struct{}, chan Line) {
	return sp.cancelCh, sp.outputCh
}

func acceptPipeline(cancel chan struct{}, in chan Line, out chan Line, pc *PipelineComponent) {
	tracer.Printf("acceptPipeline: START")
	defer tracer.Printf("acceptPipeline: END")
	defer close(out)
	for {
		select {
		case <-cancel:
			tracer.Printf("acceptPipeline: detected cancel request. Bailing out")
			return
		case l, ok := <-in:
			if l == nil && !ok {
				tracer.Printf("acceptPipeline: detected end of input. Bailing out")
				if pc.onEnd != nil {
					pc.onEnd()
				}
				return
			}
			tracer.Printf("acceptPipeline: forwarding to callback")
			if ll, err := pc.onIncomingLine(l); err == nil {
				tracer.Printf("acceptPipeline: forwarding to out channel")
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
	GetRawLineIndexAt(int) (int, error)
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
		tracer.Printf("RawLineBuffer.Replay (goroutine): START")
		defer tracer.Printf("RawLineBuffer.Replay (goroutine): END")
		tracer.Printf("RawLineBuffer.Replay (goroutine): Going to replay %d lines", len(rlb.lines))

		defer close(rlb.outputCh)
		for _, l := range rlb.lines {
			tracer.Printf("RawLineBuffer: Replaying %#v\n", l)
			rlb.outputCh <- l
		}
	}()
	return nil
}

func (rlb *RawLineBuffer) Accept(p Pipeliner) {
	cancelCh, incomingCh := p.Pipeline()
	rlb.cancelCh = cancelCh
	rlb.outputCh = make(chan Line)
	go acceptPipeline(cancelCh, incomingCh, rlb.outputCh,
		&PipelineComponent{ rlb.Append, rlb.onEnd })
}

func (rlb *RawLineBuffer) Append(l Line) (Line, error) {
	if rlb.capacity > 0 && len(rlb.lines) > rlb.capacity {
		diff := len(rlb.lines) - rlb.capacity
		// TODO Notify that we're invalidating these lines

		// Golang's version of array realloc
		rlb.lines = append(append([]Line(nil), rlb.lines[diff:]...), l)
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

func (r RawLineBuffer) GetRawLineIndexAt(i int) (int, error) {
	return i, nil
}

// LineAt returns the line at index `i`
func (r RawLineBuffer) LineAt(i int) (Line, error) {
	if i < 0 || len(r.lines) <= i {
		return nil, ErrBufferOutOfRange
	}
	return r.lines[i], nil
}

// Size returns the number of lines in the buffer
func (r RawLineBuffer) Size() int {
	return len(r.lines)
}

func (r *RawLineBuffer) SetCapacity(capacity int) {
	if capacity < 0 {
		capacity = 0
	}
	r.capacity = capacity
}

func (r RawLineBuffer) InvalidateUpTo(_ int) {
	// no op
}

func (r *RawLineBuffer) AppendLine(l Line) (Line, error) {
	return r.Append(l)
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
		src:       src,
		selection: []int{},
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
		&PipelineComponent{flb.Append, nil})
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

func (fb *FilteredLineBuffer) GetRawLineIndexAt(i int) (int, error) {
	cur := fb
	x := i
	for {
		switch cur.src.(type) {
		case *RawLineBuffer:
			return cur.selection[x], nil
		case *FilteredLineBuffer:
			x = cur.selection[x]
			cur = fb.src.(*FilteredLineBuffer)
		default:
			return -1, ErrBufferOutOfRange
		}
	}
}

// LineAt returns the line at index `i`. Note that the i-th element
// in this filtered buffer may actually correspond to a totally
// different line number in the source buffer.
func (fb FilteredLineBuffer) LineAt(i int) (Line, error) {
	if i < 0 || i >= len(fb.selection) {
		return nil, ErrBufferOutOfRange
	}
	return fb.src.LineAt(fb.selection[i])
}

// Size returns the number of lines in the buffer
func (fb FilteredLineBuffer) Size() int {
	return len(fb.selection)
}

func (fb *FilteredLineBuffer) SelectSourceLineAt(i int) {
	fb.selection = append(fb.selection, i)
}

func (flb *FilteredLineBuffer) Register(lb LineBuffer) {
	flb.buffers.Register(lb)
}

func (flb *FilteredLineBuffer) Unregister(lb LineBuffer) {
	flb.buffers.Unregister(lb)
}

type MatchFilteredLineBuffer struct {
	FilteredLineBuffer
	// Sheeeesh!
	matches [][][]int
}

func (mflb *MatchFilteredLineBuffer) SelectMatchedSourceLineAt(i int, m [][]int) {
	mflb.FilteredLineBuffer.SelectSourceLineAt(i)
	mflb.matches = append(mflb.matches, m)
}
