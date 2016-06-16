package peco

import (
	"bufio"
	"io"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/peco/peco/pipeline"
	"github.com/pkg/errors"
)

// ErrBufferOutOfRange is returned when the index within the buffer that
// was queried was out of the containing buffer's range
var ErrBufferOutOfRange = errors.New("error: Specified index is out of range")

func NewFilteredBuffer(src Buffer, page, perPage int) *FilteredBuffer {
	fb := FilteredBuffer{
		src: src,
	}

  s := perPage * (page - 1)
  if s > src.Size() {
    return &fb
  }

	selection := make([]int, 0, src.Size())
  e := s + perPage
  if e >= src.Size() {
    e = src.Size()
  }

	for i := s; i < e; i++ {
		selection = append(selection, i)
	}
	fb.selection = selection

	return &fb
}

func (flb *FilteredBuffer) Append(l Line) (Line, error) {
	return l, nil
}

// LineAt returns the line at index `i`. Note that the i-th element
// in this filtered buffer may actually correspond to a totally
// different line number in the source buffer.
func (flb FilteredBuffer) LineAt(i int) (Line, error) {
	if i >= int(len(flb.selection)) {
		return nil, ErrBufferOutOfRange
	}
	return flb.src.LineAt(flb.selection[i])
}

// Size returns the number of lines in the buffer
func (flb FilteredBuffer) Size() int {
	return len(flb.selection)
}

func NewMemoryBuffer() *MemoryBuffer {
	return &MemoryBuffer{
		done: make(chan struct{}),
	}
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

	return int(len(mb.lines))
}

func (mb *MemoryBuffer) Reset() {
	mb.done = make(chan struct{})
	mb.lines = []Line(nil)
}

func (mb MemoryBuffer) Done() <-chan struct{} {
	return mb.done
}

func (mb *MemoryBuffer) Accept(ctx context.Context, p pipeline.Producer) {
	trace("START MemoryBuffer.Accept")
	defer trace("END MemoryBuffer.Accept")
	defer close(mb.done)

  for {
    select {
    case <-ctx.Done():
			trace("MemoryBuffer received context done")
      return
    case v := <-p.OutCh():
			switch v.(type) {
			case error:
        if pipeline.IsEndMark(v.(error)) {
					trace("MemoryBuffer received end mark (read %d lines)", mb.Size())
          return
        }
      case Line:
				trace("MemoryBuffer received new line")
				mb.lines = append(mb.lines, v.(Line))
			default:
				trace("MemoryBuffer received something else %s", v)
			}
    }
  }
}

func (mb MemoryBuffer) LineAt(n int) (Line, error) {
	l := mb.locker()
	l.Lock()
	defer l.Unlock()

	if s := mb.Size(); s <= 0 || n >= s {
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
		refresh := make(chan struct{}, 1)
		defer close(done)
		defer close(refresh)

		draw := func(state *Peco, refresh chan struct{}) {
			run := false
			for loop := true; loop; {
				select {
				case _, ok := <-refresh:
					run = true
					loop = ok
				default:
					loop = false
				}
			}
			if !run {
				return
			}
			// Not a great thing to do, allowing nil to be passed
			// as state, but for testing I couldn't come up with anything
			// better for the moment
			if state != nil && !state.ExecQuery() {
				state.Hub().SendDraw(false)
			}
		}

		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-done:
					draw(state, refresh)
					return
				case <-ticker.C:
					draw(state, refresh)
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

		readCount := 0
		for scanner.Scan() {
			txt := scanner.Text()
			readCount++
			s.lines = append(s.lines, NewRawLine(txt, s.enableSep))
			notify.Do(notifycb)

			go func() {
				defer func() { recover() }()
				refresh <- struct{}{}
			}()
		}

		// XXX Just in case scanner.Scan() did not return a single line...
		// Note: this will be a no-op if notify.Do has been called before
		notify.Do(notifycb)

		trace("Read all %d lines from source", readCount)
	})
}

// Start starts
func (s *Source) Start(ctx context.Context) {
	trace("START Source.Start")
	defer trace("END Source.Start")
	defer s.OutputChannel.SendEndMark("end of input")

	s.done = make(chan struct{})

	trace("Going to send %d lines", len(s.lines))
	for _, l := range s.lines {
		select {
		case <-ctx.Done():
			trace("Source received done")
			return
		case s.OutputChannel <- l:
			trace("Source sent to output channel")
			// no op
		}
	}
}

// Ready returns the "input ready" channel. It will be closed as soon as
// the first line of input is processed via Setup()
func (s *Source) Ready() <-chan struct{} {
	return s.ready
}
