package peco

import (
	"bufio"
	"io"
	"runtime"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/peco/peco/pipeline"
	"github.com/pkg/errors"
)

// ErrBufferOutOfRange is returned when the index within the buffer that
// was queried was out of the containing buffer's range
var ErrBufferOutOfRange = errors.New("error: Specified index is out of range")

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
			select {
			case rlb.outputCh <- l:
				replayed++
			case <-rlb.cancelCh:
				return
			}
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

// Buffer interface is used for containers for lines to be
// processed by peco.
type Buffer interface {
	LineAt(int) (Line, error)
	Size() int
}

// MemoryBuffer is an implementation of Buffer
type MemoryBuffer struct {
	lines []Line
	mutex sync.Mutex
}

// XXX go through an accessor that returns a reference so that
// we are sure we are accessing/modifying the same mutex
func (mb MemoryBuffer) locker() *sync.Mutex {
	return &mb.mutex
}

func (mb MemoryBuffer) Size() int {
	l := mb.locker()
	l.Lock()
	defer l.Unlock()

	return len(mb.lines)
}

func (mb MemoryBuffer) LineAt(n int) (Line, error) {
	l := mb.locker()
	l.Lock()
	defer l.Unlock()

	if n > mb.Size() || n < 0 {
		return nil, errors.New("empty buffer")
	}

	return mb.lines[n], nil
}

// Source implements pipline.Source, and is the buffer for the input
type Source struct {
	pipeline.OutputChannel
	MemoryBuffer

	in        io.Reader
	enableSep bool
	ready     chan struct{}
	setupOnce sync.Once
}

// Creates a new Source. Does not start processing the input until you
// call Setup()
func NewSource(in io.Reader, enableSep bool) *Source {
	return &Source{
		in:            in, // Note that this may be closed, so do not rely on it
		enableSep:     enableSep,
		ready:         make(chan struct{}),
		setupOnce:     sync.Once{},
		OutputChannel: pipeline.OutputChannel(make(chan interface{})),
	}
}

// Setup reads from the input os.File.
func (s *Source) Setup(state *Peco) {
	s.setupOnce.Do(func() {
		l := s.locker()
		l.Lock()
		defer l.Unlock()

		done := make(chan struct{})
		refresh := make(chan struct{})
		defer close(done)
		defer close(refresh)

		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if _, ok := <-refresh; ok {
						// Not a great thing to do, allowing nil to be passed
						// as state, but for testing I couldn't come up with anything
						// better for the moment
						if state != nil && !state.ExecQuery() {
							state.Hub().SendDraw(false)
						}
					}
				case <-done:
					return
				}
			}
		}()

		// This sync.Once var is used to receive the notification
		// that there was at least 1 line read from the source
		var notify sync.Once
		notifycb := func() {
			// close the ready channel so others can be notified
			// that there's at least 1 line in the buffer
			close(s.ready)
		}
		scanner := bufio.NewScanner(s.in)
		for scanner.Scan() {
			s.lines = append(s.lines, NewRawLine(scanner.Text(), s.enableSep))
			notify.Do(notifycb)

			refresh <- struct{}{}
		}
	})
}

// Start starts
func (s *Source) Start(ctx context.Context) {
	go func() {
		defer s.OutputChannel.SendEndMark("end of input")

		for i := 0; i < len(s.lines); i++ {
			select {
			case <-ctx.Done():
				return
			case s.OutputChannel <- s.lines[i]:
				// no op
			}
		}
	}()
}

// Ready returns the "input ready" channel. It will be closed as soon as
// the first line of input is processed via Setup()
func (s *Source) Ready() <-chan struct{} {
	return s.ready
}
