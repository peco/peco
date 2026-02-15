package peco

import (
	"context"
	"sync"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	pdebug "github.com/lestrrat-go/pdebug"
	"github.com/mattn/go-runewidth"
	"github.com/peco/peco/internal/keyseq"
	"github.com/pkg/errors"
)

// Termbox implements the Screen interface using tcell/v2.
// The name is kept for compatibility with the rest of the codebase.
type Termbox struct {
	mutex     sync.Mutex
	screen    tcell.Screen
	resumeCh  chan chan struct{}
	suspendCh chan struct{}
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
			// to match termbox behavior expected by doAcceptChar
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

func (t *Termbox) Init(cfg *Config) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return errors.Wrap(err, "failed to create tcell screen")
	}

	if err := screen.Init(); err != nil {
		return errors.Wrap(err, "failed to initialize tcell screen")
	}

	t.screen = screen
	return t.PostInit(cfg)
}

func NewTermbox() *Termbox {
	return &Termbox{
		suspendCh: make(chan struct{}),
		resumeCh:  make(chan chan struct{}),
	}
}

func (t *Termbox) Close() error {
	if pdebug.Enabled {
		pdebug.Printf("Termbox: Close")
	}
	t.mutex.Lock()
	s := t.screen
	t.screen = nil
	t.mutex.Unlock()

	if s != nil {
		s.Fini()
	}
	return nil
}

func (t *Termbox) SetCursor(x, y int) {
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
func (t *Termbox) SendEvent(_ Event) {
	// no op
}

// Flush calls tcell's Show to synchronize the screen
func (t *Termbox) Flush() error {
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
func (t *Termbox) Sync() {
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
func (t *Termbox) PollEvent(ctx context.Context, cfg *Config) chan Event {
	evCh := make(chan Event)

	go func() {
		// keep listening to suspend requests here
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.suspendCh:
				if pdebug.Enabled {
					pdebug.Printf("poll event suspended!")
				}
				t.Close()
			}
		}
	}()

	go func() {
		defer func() { recover() }()
		defer func() { close(evCh) }()

		for {
			t.mutex.Lock()
			s := t.screen
			t.mutex.Unlock()

			if s == nil {
				// Screen finalized, treat as suspend/interrupt
				select {
				case <-ctx.Done():
					return
				case replyCh := <-t.resumeCh:
					t.Init(cfg)
					close(replyCh)
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
				case replyCh := <-t.resumeCh:
					t.Init(cfg)
					close(replyCh)
				}
				continue
			}

			evCh <- tcellEventToEvent(ev)
		}
	}()
	return evCh
}

func (t *Termbox) Suspend() {
	select {
	case t.suspendCh <- struct{}{}:
	default:
	}
}

func (t *Termbox) Resume() {
	// Resume must be a block operation, because we can't safely proceed
	// without actually knowing that the screen has been re-initialized.
	// So we send a channel where we expect a reply back, and wait for that
	ch := make(chan struct{})
	select {
	case t.resumeCh <- ch:
	default:
	}

	<-ch
}

// SetCell writes to the terminal
func (t *Termbox) SetCell(x, y int, ch rune, fg, bg Attribute) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.screen == nil {
		return
	}
	style := attributeToTcellStyle(fg, bg)
	t.screen.SetContent(x, y, ch, nil, style)
}

// Size returns the dimensions of the current terminal
func (t *Termbox) Size() (int, int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.screen == nil {
		return 0, 0
	}
	return t.screen.Size()
}

type PrintArgs struct {
	X       int
	XOffset int
	Y       int
	Fg      Attribute
	Bg      Attribute
	Msg     string
	Fill    bool
}

func (t *Termbox) Print(args PrintArgs) int {
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
	for len(msg) > 0 {
		c, w := utf8.DecodeRuneInString(msg)
		if c == utf8.RuneError {
			c = '?'
			w = 1
		}
		msg = msg[w:]
		if c == '\t' {
			// In case we found a tab, we draw it as 4 spaces
			n := 4 - (x+xOffset)%4
			for i := int(0); i <= n; i++ {
				t.SetCell(int(x+i), int(y), ' ', fg, bg)
			}
			written += n
			x += n
		} else {
			t.SetCell(int(x), int(y), c, fg, bg)
			n := int(runewidth.RuneWidth(c))
			x += n
			written += n
		}
	}

	if !args.Fill {
		return written
	}

	width, _ := t.Size()
	for ; x < int(width); x++ {
		t.SetCell(int(x), int(y), ' ', fg, bg)
	}
	written += int(width) - x
	return written
}
