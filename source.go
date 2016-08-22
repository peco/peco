package peco

import (
	"bufio"
	"io"
	"sync"
	"time"

	"github.com/lestrrat/go-pdebug"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/pipeline"
	"golang.org/x/net/context"
)

// Creates a new Source. Does not start processing the input until you
// call Setup()
func NewSource(in io.Reader, idgen lineIDGenerator, enableSep bool) *Source {
	s := &Source{
		in:            in, // Note that this may be closed, so do not rely on it
		enableSep:     enableSep,
		idgen:         idgen,
		ready:         make(chan struct{}),
		setupDone:     make(chan struct{}),
		setupOnce:     sync.Once{},
		OutputChannel: pipeline.OutputChannel(make(chan interface{})),
	}
	s.Reset()
	return s
}

// Setup reads from the input os.File.
func (s *Source) Setup(state *Peco) {
	s.setupOnce.Do(func() {
		done := make(chan struct{})
		refresh := make(chan struct{}, 1)
		defer close(done)
		defer close(refresh)

		draw := func(state *Peco) {
			// Not a great thing to do, allowing nil to be passed
			// as state, but for testing I couldn't come up with anything
			// better for the moment
			if state != nil {
				state.Hub().SendDraw(nil)
			}
		}

		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-done:
					draw(state)
					return
				case <-ticker.C:
					draw(state)
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
		defer func() {
			if util.IsTty(s.in) {
				return
			}
			if closer, ok := s.in.(io.Closer); ok {
				closer.Close()
			}
		}()

		readCount := 0
		for scanner.Scan() {
			txt := scanner.Text()
			readCount++
			s.Append(NewRawLine(s.idgen.next(), txt, s.enableSep))
			notify.Do(notifycb)
		}

		// XXX Just in case scanner.Scan() did not return a single line...
		// Note: this will be a no-op if notify.Do has been called before
		notify.Do(notifycb)
		// And also, close the done channel so we can tell the consumers
		// we have finished reading everything
		close(s.setupDone)

		if pdebug.Enabled {
			pdebug.Printf("Read all %d lines from source", readCount)
		}
	})
}

// Start starts
func (s *Source) Start(ctx context.Context, out pipeline.OutputChannel) {
	// I should be the only one running this method until I bail out
	if pdebug.Enabled {
		g := pdebug.Marker("Source.Start")
		defer g.End()
		defer pdebug.Printf("Source sent %d lines", len(s.lines))
	}

	for _, l := range s.lines {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("Source: context.Done detected")
			}
			return
		default:
			out.Send(l)
		}
	}
	out.SendEndMark("end of input")
}

// Reset resets the state of the source object so that it
// is ready to feed the filters
func (s *Source) Reset() {
	if pdebug.Enabled {
		g := pdebug.Marker("Source.Reset")
		defer g.End()
	}
	s.OutputChannel = pipeline.OutputChannel(make(chan interface{}))
}

// Ready returns the "input ready" channel. It will be closed as soon as
// the first line of input is processed via Setup()
func (s *Source) Ready() <-chan struct{} {
	return s.ready
}

// SetupDone returns the "read all lines" channel. It will be closed as soon as
// the all input has been read
func (s *Source) SetupDone() <-chan struct{} {
	return s.done
}

func (s *Source) LineAt(n int) (Line, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return bufferLineAt(s.lines, n)
}

func (s *Source) Size() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return bufferSize(s.lines)
}

func (s *Source) Append(l Line) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	bufferAppend(&s.lines, l)
}
