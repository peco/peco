package peco

import (
	"context"
	"unicode/utf8"

	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
)

func (t *Termbox) Init() error {
	if err := termbox.Init(); err != nil {
		return errors.Wrap(err, "failed to initialized termbox")
	}

	return t.PostInit()
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
	termbox.Interrupt()
	termbox.Close()
	return nil
}

// SendEvent is used to allow programmers generate random
// events, but it's only useful for testing purposes.
// When interactiving with termbox-go, this method is a noop
func (t *Termbox) SendEvent(_ termbox.Event) {
	// no op
}

// Flush calls termbox.Flush
func (t *Termbox) Flush() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return errors.Wrap(termbox.Flush(), "failed to flush termbox")
}

// PollEvent returns a channel that you can listen to for
// termbox's events. The actual polling is done in a
// separate gouroutine
func (t *Termbox) PollEvent(ctx context.Context) chan termbox.Event {
	// XXX termbox.PollEvent() can get stuck on unexpected signal
	// handling cases. We still would like to wait until the user
	// (termbox) has some event for us to process, but we don't
	// want to allow termbox to control/block our input loop.
	//
	// Solution: put termbox polling in a separate goroutine,
	// and we just watch for a channel. The loop can now
	// safely be implemented in terms of select {} which is
	// safe from being stuck.
	evCh := make(chan termbox.Event)

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
			ev := termbox.PollEvent()
			if ev.Type != termbox.EventInterrupt {
				evCh <- ev
				continue
			}

			select {
			case <-ctx.Done():
				return
			case replyCh := <-t.resumeCh:
				t.Init()
				close(replyCh)
			}
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
	// without actually knowing that termbox has been re-initialized.
	// So we send a channel where we expect a reply back, and wait for that
	ch := make(chan struct{})
	select {
	case t.resumeCh <- ch:
	default:
	}

	<-ch
}

// SetCell writes to the terminal
func (t *Termbox) SetCell(x, y int, ch rune, fg, bg termbox.Attribute) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	termbox.SetCell(x, y, ch, fg, bg)
}

// Size returns the dimensions of the current terminal
func (t *Termbox) Size() (int, int) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return termbox.Size()
}

type PrintArgs struct {
	X       int
	XOffset int
	Y       int
	Fg      termbox.Attribute
	Bg      termbox.Attribute
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
