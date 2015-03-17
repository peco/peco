package peco

import "github.com/nsf/termbox-go"

// Screen hides termbox from tne consuming code so that
// it can be swapped out for testing
type Screen interface {
	Flush() error
	PollEvent() chan termbox.Event
	SetCell(int, int, rune, termbox.Attribute, termbox.Attribute)
	Size() (int, int)
	SendEvent(termbox.Event)
}

// Termbox just hands out the processing to the termbox library
type Termbox struct{}

// termbox always gives us some sort of warning when we run
// go run -race cmd/peco/peco.go
var termboxMutex = newMutex()

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
	return termbox.Flush()
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
			evCh <- termbox.PollEvent()
		}
	}()
	return evCh

}

// SetCell writes to the terminal
func (t Termbox) SetCell(x, y int, ch rune, fg, bg termbox.Attribute) {
	termboxMutex.Lock()
	defer termboxMutex.Unlock()
	termbox.SetCell(x, y, ch, fg, bg)
}

// Size returns the dimensions of the current terminal
func (t Termbox) Size() (int, int) {
	termboxMutex.Lock()
	defer termboxMutex.Unlock()
	return termbox.Size()
}
