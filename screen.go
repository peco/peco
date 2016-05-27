package peco

import "github.com/nsf/termbox-go"

// termbox always gives us some sort of warning when we run
// go run -race cmd/peco/peco.go
var termboxMutex = newMutex()

// These functions are here so that we can test
var (
	termboxClose     = termbox.Close
	termboxFlush     = termbox.Flush
	termboxInit      = termbox.Init
	termboxPollEvent = termbox.PollEvent
	termboxSetCell   = termbox.SetCell
	termboxSize      = termbox.Size
)

func (t Termbox) Init() error {
	trace("initializing termbox")
	if err := termboxInit(); err != nil {
		return err
	}

	return t.PostInit()
}

func (t Termbox) Close() error {
	termboxClose()
	return nil
}

// SendEvent is used to allow programmers generate random
// events, but it's only useful for testing purposes.
// When interactiving with termbox-go, this method is a noop
func (t Termbox) SendEvent(_ termbox.Event) {
	// no op
}

// Flush calls termbox.Flush
func (t Termbox) Flush() error {
	termboxMutex.Lock()
	defer termboxMutex.Unlock()
	return termboxFlush()
}

// PollEvent returns a channel that you can listen to for
// termbox's events. The actual polling is done in a
// separate gouroutine
func (t Termbox) PollEvent() chan termbox.Event {
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
		defer func() { recover() }()
		defer func() { close(evCh) }()
		for {
			evCh <- termboxPollEvent()
		}
	}()
	return evCh

}

// SetCell writes to the terminal
func (t Termbox) SetCell(x, y int, ch rune, fg, bg termbox.Attribute) {
	termboxMutex.Lock()
	defer termboxMutex.Unlock()
	termboxSetCell(x, y, ch, fg, bg)
}

// Size returns the dimensions of the current terminal
func (t Termbox) Size() (int, int) {
	termboxMutex.Lock()
	defer termboxMutex.Unlock()
	return termboxSize()
}
