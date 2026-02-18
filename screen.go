package peco

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	pdebug "github.com/lestrrat-go/pdebug"
	"github.com/mattn/go-runewidth"
	"github.com/peco/peco/internal/ansi"
	"github.com/peco/peco/internal/keyseq"
)

// TcellScreen implements the Screen interface using tcell/v2.
type TcellScreen struct {
	mutex     sync.Mutex
	screen    tcell.Screen
	resumeCh  chan chan error
	suspendCh chan struct{}
	doneCh    chan struct{} // closed on permanent Close() to signal goroutines to exit
	closeOnce sync.Once     // ensures doneCh is closed exactly once
	errWriter io.Writer     // destination for error output (defaults to os.Stderr)
}

// tcellKeyToKeyseq maps tcell navigation/function key constants to peco keyseq constants.
var tcellKeyToKeyseq = map[tcell.Key]keyseq.KeyType{
	tcell.KeyUp:        keyseq.KeyArrowUp,
	tcell.KeyDown:      keyseq.KeyArrowDown,
	tcell.KeyLeft:      keyseq.KeyArrowLeft,
	tcell.KeyRight:     keyseq.KeyArrowRight,
	tcell.KeyInsert:    keyseq.KeyInsert,
	tcell.KeyDelete:    keyseq.KeyDelete,
	tcell.KeyHome:      keyseq.KeyHome,
	tcell.KeyEnd:       keyseq.KeyEnd,
	tcell.KeyPgUp:      keyseq.KeyPgup,
	tcell.KeyPgDn:      keyseq.KeyPgdn,
	tcell.KeyF1:        keyseq.KeyF1,
	tcell.KeyF2:        keyseq.KeyF2,
	tcell.KeyF3:        keyseq.KeyF3,
	tcell.KeyF4:        keyseq.KeyF4,
	tcell.KeyF5:        keyseq.KeyF5,
	tcell.KeyF6:        keyseq.KeyF6,
	tcell.KeyF7:        keyseq.KeyF7,
	tcell.KeyF8:        keyseq.KeyF8,
	tcell.KeyF9:        keyseq.KeyF9,
	tcell.KeyF10:       keyseq.KeyF10,
	tcell.KeyF11:       keyseq.KeyF11,
	tcell.KeyF12:       keyseq.KeyF12,
	tcell.KeyBackspace: keyseq.KeyBackspace,
	tcell.KeyTab:       keyseq.KeyTab,
	tcell.KeyEnter:     keyseq.KeyEnter,
	tcell.KeyEscape:    keyseq.KeyEsc,
	tcell.KeyBacktab:   keyseq.KeyTab, // Shift+Tab → Tab for compatibility
}

// tcellEventToEvent converts a tcell.Event to peco's internal Event type.
func tcellEventToEvent(tev tcell.Event) Event {
	switch ev := tev.(type) {
	case *tcell.EventKey:
		var mod keyseq.ModifierKey
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			mod |= keyseq.ModCtrl
		}
		if ev.Modifiers()&tcell.ModShift != 0 {
			mod |= keyseq.ModShift
		}
		if ev.Modifiers()&tcell.ModAlt != 0 {
			mod |= keyseq.ModAlt
		}

		key := ev.Key()

		// Rune keys (printable characters)
		if key == tcell.KeyRune {
			r := ev.Rune()
			// Special case: space must be sent as KeySpace with Ch=0
			// to match the convention expected by doAcceptChar
			if r == ' ' {
				return Event{
					Type: EventKey,
					Key:  keyseq.KeySpace,
					Ch:   0,
					Mod:  mod,
				}
			}
			return Event{
				Type: EventKey,
				Key:  0,
				Ch:   r,
				Mod:  mod,
			}
		}

		// Navigation/function keys via lookup table
		if mapped, ok := tcellKeyToKeyseq[key]; ok {
			return Event{
				Type: EventKey,
				Key:  mapped,
				Ch:   0,
				Mod:  mod,
			}
		}

		// Ctrl keys (0x00-0x1F) and DEL (0x7F) — tcell uses the same
		// ASCII control code values as peco's keyseq, so direct cast works.
		if key <= 0x1F || key == 0x7F {
			return Event{
				Type: EventKey,
				Key:  keyseq.KeyType(key),
				Ch:   0,
				Mod:  mod,
			}
		}

		// Fallback: treat as error
		return Event{Type: EventError}

	case *tcell.EventResize:
		return Event{Type: EventResize}

	default:
		return Event{Type: EventError}
	}
}

// attributeToTcellColor converts a peco Attribute to a tcell.Color.
func attributeToTcellColor(attr Attribute) tcell.Color {
	if attr&AttrTrueColor != 0 {
		rgb := attr & 0x00FFFFFF
		return tcell.NewHexColor(int32(rgb))
	}
	colorVal := attr & 0x01FF
	if colorVal == 0 {
		return tcell.ColorDefault
	}
	return tcell.PaletteColor(int(colorVal - 1))
}

// attributeToTcellStyle converts peco Attribute fg/bg values to a tcell.Style.
func attributeToTcellStyle(fg, bg Attribute) tcell.Style {
	style := tcell.StyleDefault.
		Foreground(attributeToTcellColor(fg)).
		Background(attributeToTcellColor(bg))

	// Extract style attributes from both fg and bg
	attrs := fg | bg
	if attrs&AttrBold != 0 {
		style = style.Bold(true)
	}
	if attrs&AttrUnderline != 0 {
		style = style.Underline(true)
	}
	if attrs&AttrReverse != 0 {
		style = style.Reverse(true)
	}

	return style
}

func (t *TcellScreen) Init(_ *Config) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("failed to create tcell screen: %w", err)
	}

	if err := screen.Init(); err != nil {
		return fmt.Errorf("failed to initialize tcell screen: %w", err)
	}

	t.screen = screen
	return nil
}

func NewTcellScreen() *TcellScreen {
	return &TcellScreen{
		suspendCh: make(chan struct{}),
		resumeCh:  make(chan chan error),
		doneCh:    make(chan struct{}),
		errWriter: os.Stderr,
	}
}

// finiScreen finalizes the tcell screen without signaling a permanent
// shutdown. Used by the suspend handler so the goroutine continues
// to listen for further suspend/resume cycles.
func (t *TcellScreen) finiScreen() {
	t.mutex.Lock()
	s := t.screen
	t.screen = nil
	t.mutex.Unlock()

	if s != nil {
		s.Fini()
	}
}

// Close permanently shuts down the screen and signals all goroutines
// started by PollEvent to exit.
func (t *TcellScreen) Close() error {
	if pdebug.Enabled {
		pdebug.Printf("TcellScreen: Close")
	}
	t.finiScreen()
	t.closeOnce.Do(func() { close(t.doneCh) })
	return nil
}

func (t *TcellScreen) SetCursor(x, y int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.screen == nil {
		return
	}
	t.screen.ShowCursor(x, y)
}

// SendEvent is used to allow programmers generate random
// events, but it's only useful for testing purposes.
// When interacting with tcell, this method is a noop
func (t *TcellScreen) SendEvent(_ Event) {
	// no op
}

// Flush calls tcell's Show to synchronize the screen
func (t *TcellScreen) Flush() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.screen == nil {
		return nil
	}
	t.screen.Show()
	return nil
}

// Sync forces a complete redraw of every cell on the physical display.
// This recovers from screen corruption caused by external output (e.g.,
// STDERR messages written directly to the terminal).
func (t *TcellScreen) Sync() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.screen == nil {
		return
	}
	t.screen.Sync()
}

// PollEvent returns a channel that you can listen to for
// terminal events. The actual polling is done in a
// separate goroutine
func (t *TcellScreen) PollEvent(ctx context.Context, cfg *Config) chan Event {
	evCh := make(chan Event)

	go func() {
		// keep listening to suspend requests here
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.doneCh:
				return
			case <-t.suspendCh:
				if pdebug.Enabled {
					pdebug.Printf("poll event suspended!")
				}
				t.finiScreen()
			}
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(t.errWriter, "peco: panic in PollEvent goroutine: %v\n%s", r, debug.Stack())
			}
			close(evCh)
		}()

		for {
			t.mutex.Lock()
			s := t.screen
			t.mutex.Unlock()

			if s == nil {
				// Screen finalized, treat as suspend/interrupt
				select {
				case <-ctx.Done():
					return
				case <-t.doneCh:
					return
				case replyCh := <-t.resumeCh:
					if err := t.Init(cfg); err != nil {
						fmt.Fprintf(t.errWriter, "peco: failed to re-initialize screen on resume: %v\n", err)
						replyCh <- err
					} else {
						replyCh <- nil
					}
					continue
				}
			}

			ev := s.PollEvent()
			if ev == nil {
				// PollEvent returns nil when screen is finalized.
				// Wait for resume or context cancellation.
				select {
				case <-ctx.Done():
					return
				case <-t.doneCh:
					return
				case replyCh := <-t.resumeCh:
					if err := t.Init(cfg); err != nil {
						fmt.Fprintf(t.errWriter, "peco: failed to re-initialize screen on resume: %v\n", err)
						replyCh <- err
					} else {
						replyCh <- nil
					}
				}
				continue
			}

			evCh <- tcellEventToEvent(ev)
		}
	}()
	return evCh
}

func (t *TcellScreen) Suspend() {
	select {
	case t.suspendCh <- struct{}{}:
	default:
	}
}

func (t *TcellScreen) Resume(ctx context.Context) error {
	// Resume must be a block operation, because we can't safely proceed
	// without actually knowing that the screen has been re-initialized.
	// So we send a channel where we expect a reply back, and wait for that.
	//
	// Both selects are guarded by ctx.Done() to avoid deadlock: if the
	// polling goroutine is not yet waiting on resumeCh, a non-blocking
	// send would silently drop the message and the subsequent receive
	// would block forever.
	ch := make(chan error, 1)
	select {
	case t.resumeCh <- ch:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SetCell writes to the terminal
func (t *TcellScreen) SetCell(x, y int, ch rune, fg, bg Attribute) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.screen == nil {
		return
	}
	style := attributeToTcellStyle(fg, bg)
	t.screen.SetContent(x, y, ch, nil, style)
}

// Size returns the dimensions of the current terminal
func (t *TcellScreen) Size() (int, int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.screen == nil {
		return 0, 0
	}
	return t.screen.Size()
}

type PrintArgs struct {
	X         int
	XOffset   int
	Y         int
	Fg        Attribute
	Bg        Attribute
	Msg       string
	Fill      bool
	ANSIAttrs []ansi.AttrSpan // per-character ANSI attributes for this segment
}

func (t *TcellScreen) Print(args PrintArgs) int {
	return screenPrint(t, args)
}

func screenPrint(t Screen, args PrintArgs) int {
	var written int

	bg := args.Bg
	fg := args.Fg
	msg := args.Msg
	x := args.X
	y := args.Y
	xOffset := args.XOffset

	// ANSI span tracking
	ansiAttrs := args.ANSIAttrs
	spanIdx := 0
	spanPos := 0

	for len(msg) > 0 {
		c, w := utf8.DecodeRuneInString(msg)
		if c == utf8.RuneError {
			c = '?'
			w = 1
		}
		msg = msg[w:]

		// Determine effective fg/bg for this character
		efg, ebg := fg, bg
		if ansiAttrs != nil && spanIdx < len(ansiAttrs) {
			span := ansiAttrs[spanIdx]
			if Attribute(span.Fg) != ColorDefault {
				efg = Attribute(span.Fg)
			}
			if Attribute(span.Bg) != ColorDefault {
				ebg = Attribute(span.Bg)
			}
			spanPos++
			if spanPos >= span.Length {
				spanIdx++
				spanPos = 0
			}
		}

		if c == '\t' {
			// In case we found a tab, we draw it as spaces up to the next tab stop
			n := 4 - (x+xOffset)%4
			for i := range n {
				t.SetCell(x+i, y, ' ', efg, ebg)
			}
			written += n
			x += n
		} else {
			t.SetCell(x, y, c, efg, ebg)
			n := runewidth.RuneWidth(c)
			x += n
			written += n
		}
	}

	if !args.Fill {
		return written
	}

	width, _ := t.Size()
	for ; x < width; x++ {
		t.SetCell(x, y, ' ', fg, bg)
	}
	written += width - x
	return written
}
