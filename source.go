package peco

import (
	"bufio"
	"context"
	"io"
	"sync"
	"time"

	"github.com/lestrrat-go/pdebug/v2"
	"github.com/peco/peco/buffer"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// Creates a new Source. Does not start processing the input until you
// call Setup()
func NewSource(name string, in io.Reader, isInfinite bool, idgen line.IDGenerator, capacity int, enableSep bool) *Source {
	s := &Source{
		name:       name,
		capacity:   capacity,
		enableSep:  enableSep,
		idgen:      idgen,
		in:         in, // Note that this may be closed, so do not rely on it
		inClosed:   false,
		isInfinite: isInfinite,
		ready:      make(chan struct{}),
		setupDone:  make(chan struct{}),
		ChanOutput: pipeline.ChanOutput(make(chan interface{})),
	}
	s.Reset()
	return s
}

func (s *Source) Name() string {
	return s.name
}

func (s *Source) IsInfinite() bool {
	return s.isInfinite && !s.inClosed
}

// Setup reads from the input os.File.
func (s *Source) Setup(ctx context.Context, state *Peco) {
	s.setupOnce.Do(func() {
		done := make(chan struct{})
		refresh := make(chan struct{}, 1)
		defer close(done)
		defer close(refresh)
		// And also, close the done channel so we can tell the consumers
		// we have finished reading everything
		defer close(s.setupDone)

		draw := func(state *Peco) {
			state.Hub().SendDraw(ctx, nil)
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
		// This is wrapped in a sync.Notify so we can safely call
		// it in multiple places
		var notify sync.Once
		notifycb := func() {
			// close the ready channel so others can be notified
			// that there's at least 1 line in the buffer
			state.Hub().SendStatusMsg(ctx, "")
			close(s.ready)
		}

		// Register this to be called in a defer, just in case we could bailed
		// out without reading a single line.
		// Note: this will be a no-op if notify.Do has been called before
		defer notify.Do(notifycb)

		if pdebug.Enabled {
			pdebug.Printf(ctx, "Source: using buffer size of %dkb", state.maxScanBufferSize)
		}
		scanbuf := make([]byte, state.maxScanBufferSize*1024)
		scanner := bufio.NewScanner(s.in)
		scanner.Buffer(scanbuf, state.maxScanBufferSize*1024)
		defer func() {
			if util.IsTty(s.in) {
				return
			}
			if closer, ok := s.in.(io.Closer); ok {
				s.inClosed = true
				closer.Close()
			}
		}()

		lines := make(chan string)
		go func() {
			var scanned int
			if pdebug.Enabled {
				defer func() { pdebug.Printf(ctx, "Source scanned %d lines", scanned) }()
			}

			defer close(lines)
			for scanner.Scan() {
				newLine := scanner.Text()
				select {
				case <-ctx.Done():
					if pdebug.Enabled {
						pdebug.Printf(ctx, "Bailing out of source setup text reader loop, because ctx was canceled")
					}
					return
				case lines <- newLine:
				}
				scanned++
			}
		}()

		state.Hub().SendStatusMsg(ctx, "Waiting for input...")

		readCount := 0
		for loop := true; loop; {
			select {
			case <-ctx.Done():
				if pdebug.Enabled {
					pdebug.Printf(ctx, "Bailing out of source setup, because ctx was canceled")
				}
				return
			case l, ok := <-lines:
				if !ok {
					if pdebug.Enabled {
						pdebug.Printf(ctx, "No more lines to read...")
					}
					loop = false
					break
				}

				readCount++
				s.Append(line.NewRaw(s.idgen.Next(), l, s.enableSep))
				notify.Do(notifycb)
			}
		}

		if pdebug.Enabled {
			pdebug.Printf(ctx, "Read all %d lines from source", readCount)
		}
	})
}

// Start starts
func (s *Source) Start(ctx context.Context, out pipeline.ChanOutput) {
	var sent int
	// I should be the only one running this method until I bail out
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "Source.Start (%d lines in buffer)", len(s.lines))
		defer g.End()
		defer func() { pdebug.Printf(ctx, "Source sent %d lines", sent) }()
	}
	defer func() { _ = out.SendEndMark("end of input") }()

	var resume bool
	select {
	case <-s.setupDone:
	default:
		resume = true
	}

	if !resume {
		// no fancy resume handling needed. just go
		for _, l := range s.lines {
			select {
			case <-ctx.Done():
				if pdebug.Enabled {
					pdebug.Printf(ctx, "Source: context.Done detected")
				}
				return
			default:
				_ = out.Send(l)
				sent++
			}
		}
		return
	}

	// For the first time we get called, we may possibly be in the
	// middle of reading a really long input stream. In this case,
	// we should resume where we left off.

	var prev = 0
	var setupDone bool
	for {
		// This is where we are ready up to
		upto := s.Size()
		// We bail out if we are done with the setup, and our
		// buffer has not grown
		if setupDone && upto == prev {
			return
		}

		for i := prev; i < upto; i++ {
			select {
			case <-ctx.Done():
				if pdebug.Enabled {
					pdebug.Printf(ctx, "Source: context.Done detected")
				}
				return
			default:
				l, _ := s.LineAt(i)
				_ = out.Send(l)
				sent++
			}
		}
		// Remember how far we have processed
		prev = upto

		// Check if we're done with setup
		select {
		case <-s.setupDone:
			setupDone = true
		default:
		}

	}
}

// Reset resets the state of the source object so that it
// is ready to feed the filters
func (s *Source) Reset() {
	if pdebug.Enabled {
		g := pdebug.Marker(context.TODO(), "Source.Reset")
		defer g.End()
	}
	s.ChanOutput = pipeline.ChanOutput(make(chan interface{}))
}

// Ready returns the "input ready" channel. It will be closed as soon as
// the first line of input is processed via Setup()
func (s *Source) Ready() <-chan struct{} {
	return s.ready
}

// SetupDone returns the "read all lines" channel. It will be closed as soon as
// the all input has been read
func (s *Source) SetupDone() <-chan struct{} {
	return s.setupDone
}

func (s *Source) LinesInRange(start, end int) []line.Line {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.lines[start:end]
}

func (s *Source) LineAt(n int) (line.Line, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return buffer.LineAt(s.lines, n)
}

func (s *Source) Size() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.lines)
}

func (s *Source) Append(l line.Line) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.lines = append(s.lines, l)
	if s.capacity > 0 && len(s.lines) > s.capacity {
		diff := len(s.lines) - s.capacity

		// Golang's version of array realloc
		s.lines = s.lines[diff:s.capacity:s.capacity]
	}
}
