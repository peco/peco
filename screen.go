package peco

import (
	"context"
	"sync"
	"unicode/utf8"

	pdebug "github.com/lestrrat-go/pdebug"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
)

func (t *Termbox) Init(cfg *Config) error {
	if err := termbox.Init(); err != nil {
		return errors.Wrap(err, "failed to initialized termbox")
	}

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
	termbox.Interrupt()
	termbox.Close()
	return nil
}

func (t *Termbox) SetCursor(x, y int) {
	termbox.SetCursor(x, y)
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
func (t *Termbox) PollEvent(ctx context.Context, cfg *Config) chan termbox.Event {
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
				t.Init(cfg)
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

type PrintCmd struct {
	screen  Screen
	x       int
	xOffset int
	y       int
	style   *Style
	msg     string
	fill    bool
}

var printCmdPool = sync.Pool{
	New: func() interface{} { return &PrintCmd{} },
}

func (t *Termbox) Print(msg string) *PrintCmd {
	cmd := printCmdPool.Get().(*PrintCmd)
	cmd.msg = msg
	cmd.screen = t
	return cmd
}

func (cmd *PrintCmd) Release() {
	cmd.x = 0
	cmd.xOffset = 0
	cmd.y = 0
	cmd.style = nil
	cmd.msg = ""
	cmd.fill = false
	printCmdPool.Put(cmd)
}

func (cmd *PrintCmd) X(v int) *PrintCmd {
	cmd.x = v
	return cmd
}

func (cmd *PrintCmd) XOffset(v int) *PrintCmd {
	cmd.xOffset = v
	return cmd
}

func (cmd *PrintCmd) Y(v int) *PrintCmd {
	cmd.y = v
	return cmd
}

func (cmd *PrintCmd) Style(s *Style) *PrintCmd {
	cmd.style = s
	return cmd
}

func (cmd *PrintCmd) Fill(v bool) *PrintCmd {
	cmd.fill = v
	return cmd
}

func (cmd *PrintCmd) Do() int {
	defer cmd.Release()

	var written int

	fg := cmd.style.fg | cmd.style.attrs
	bg := cmd.style.bg
	msg := cmd.msg
	x := cmd.x
	y := cmd.y
	xOffset := cmd.xOffset
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
				cmd.screen.SetCell(int(x+i), int(y), ' ', fg, bg)
			}
			written += n
			x += n
		} else {
			cmd.screen.SetCell(int(x), int(y), c, fg, bg)
			n := int(runewidth.RuneWidth(c))
			x += n
			written += n
		}
	}

	if !cmd.fill {
		return written
	}

	width, _ := cmd.screen.Size()
	for ; x < int(width); x++ {
		cmd.screen.SetCell(int(x), int(y), ' ', fg, bg)
	}
	written += int(width) - x
	return written
}
