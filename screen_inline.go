package peco

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"

	"github.com/gdamore/tcell/v2"
	pdebug "github.com/lestrrat-go/pdebug"
)

// InlineScreen implements the Screen interface for rendering peco in a
// portion of the terminal without using the alternate screen buffer.
// This preserves terminal scroll history above the inline region.
type InlineScreen struct {
	mutex      sync.Mutex
	screen     tcell.Screen
	heightSpec HeightSpec
	height     int // resolved line count
	yOffset    int // physical row where inline region starts

	// savedAltscreen holds the original TCELL_ALTSCREEN value so we can
	// restore it on Close.
	savedAltscreen string

	errWriter io.Writer // destination for error output (defaults to os.Stderr)
}

// NewInlineScreen creates a new InlineScreen with the given height spec.
func NewInlineScreen(spec HeightSpec) *InlineScreen {
	return &InlineScreen{
		heightSpec: spec,
		errWriter:  os.Stderr,
	}
}

func (s *InlineScreen) Init(_ *Config) error {
	// Save and override TCELL_ALTSCREEN to prevent alternate screen buffer
	s.savedAltscreen = os.Getenv("TCELL_ALTSCREEN")
	os.Setenv("TCELL_ALTSCREEN", "disable")

	screen, err := tcell.NewScreen()
	if err != nil {
		os.Setenv("TCELL_ALTSCREEN", s.savedAltscreen)
		return fmt.Errorf("failed to create tcell screen: %w", err)
	}

	if err := screen.Init(); err != nil {
		os.Setenv("TCELL_ALTSCREEN", s.savedAltscreen)
		return fmt.Errorf("failed to initialize tcell screen: %w", err)
	}

	s.mutex.Lock()
	s.screen = screen
	s.mutex.Unlock()

	termWidth, termHeight := screen.Size()
	s.height = s.heightSpec.Resolve(termHeight)
	s.yOffset = termHeight - s.height

	// Push existing terminal content up by writing newlines to the TTY,
	// then move cursor back up to the start of our region.
	if tty, ok := screen.Tty(); ok {
		buf := make([]byte, s.height)
		for i := range buf {
			buf[i] = '\n'
		}
		_, _ = tty.Write(buf)
		// Move cursor up to the start of our inline region
		fmt.Fprintf(tty, "\033[%dA", s.height)
	}

	// Lock the region above our area so tcell won't overwrite it
	screen.LockRegion(0, 0, termWidth, s.yOffset, true)

	// Clear our region
	for y := range s.height {
		for x := range termWidth {
			screen.SetContent(x, s.yOffset+y, ' ', nil, tcell.StyleDefault)
		}
	}
	screen.Show()

	return nil
}

func (s *InlineScreen) Close() error {
	if pdebug.Enabled {
		pdebug.Printf("InlineScreen: Close")
	}
	s.mutex.Lock()
	scr := s.screen
	s.screen = nil
	s.mutex.Unlock()

	if scr != nil {
		// Clear our region and position cursor at the start of it
		if tty, ok := scr.Tty(); ok {
			// Move cursor to the start of our inline region and clear from there down
			fmt.Fprintf(tty, "\033[%d;1H", s.yOffset+1) // 1-based row
			_, _ = tty.Write([]byte("\033[J"))          // clear from cursor to end of screen
		}
		scr.Fini()
	}

	// Restore original TCELL_ALTSCREEN
	if s.savedAltscreen == "" {
		os.Unsetenv("TCELL_ALTSCREEN")
	} else {
		os.Setenv("TCELL_ALTSCREEN", s.savedAltscreen)
	}

	return nil
}

func (s *InlineScreen) SetCell(x, y int, ch rune, fg, bg Attribute) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.screen == nil {
		return
	}
	style := attributeToTcellStyle(fg, bg)
	s.screen.SetContent(x, y+s.yOffset, ch, nil, style)
}

func (s *InlineScreen) SetCursor(x, y int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.screen == nil {
		return
	}
	s.screen.ShowCursor(x, y+s.yOffset)
}

func (s *InlineScreen) Print(args PrintArgs) int {
	return screenPrint(s, args)
}

func (s *InlineScreen) Flush() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.screen == nil {
		return nil
	}
	s.screen.Show()
	return nil
}

func (s *InlineScreen) Sync() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.screen == nil {
		return
	}
	s.screen.Sync()
}

// Size returns the constrained dimensions (full width, inline height).
func (s *InlineScreen) Size() (int, int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.screen == nil {
		return 0, 0
	}
	w, _ := s.screen.Size()
	return w, s.height
}

func (s *InlineScreen) PollEvent(ctx context.Context, _ *Config) chan Event {
	evCh := make(chan Event)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(s.errWriter, "peco: panic in PollEvent goroutine: %v\n%s", r, debug.Stack())
			}
			close(evCh)
		}()

		for {
			s.mutex.Lock()
			scr := s.screen
			s.mutex.Unlock()

			if scr == nil {
				return
			}

			ev := scr.PollEvent()
			if ev == nil {
				return
			}

			// On resize, recalculate height and yOffset
			if _, ok := ev.(*tcell.EventResize); ok {
				s.mutex.Lock()
				if s.screen != nil {
					termWidth, termHeight := s.screen.Size()
					s.height = s.heightSpec.Resolve(termHeight)
					s.yOffset = termHeight - s.height
					// Re-lock the region above
					s.screen.LockRegion(0, 0, termWidth, s.yOffset, true)
				}
				s.mutex.Unlock()
			}

			pecoEv := tcellEventToEvent(ev)
			select {
			case <-ctx.Done():
				return
			case evCh <- pecoEv:
			}
		}
	}()
	return evCh
}

// SendEvent is a no-op for InlineScreen (same as TcellScreen).
func (s *InlineScreen) SendEvent(_ Event) {}

// Suspend is a no-op for inline mode.
func (s *InlineScreen) Suspend() {}

// Resume is a no-op for inline mode.
func (s *InlineScreen) Resume(_ context.Context) {}
