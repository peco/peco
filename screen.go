package peco

import "github.com/nsf/termbox-go"

// Screen hides termbox from tne consuming code so that
// it can be swapped out for testing
type Screen interface {
	Clear(termbox.Attribute, termbox.Attribute) error
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

func (t Termbox) SendEvent(_ termbox.Event) {
	// no op
}

func (t Termbox) Clear(fg, bg termbox.Attribute) error {
	termboxMutex.Lock()
	defer termboxMutex.Unlock()
	return termbox.Clear(fg, bg)
}

func (t Termbox) Flush() error {
	termboxMutex.Lock()
	defer termboxMutex.Unlock()
	return termbox.Flush()
}

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

func (t Termbox) SetCell(x, y int, ch rune, fg, bg termbox.Attribute) {
	termboxMutex.Lock()
	defer termboxMutex.Unlock()
	termbox.SetCell(x, y, ch, fg, bg)
}

func (t Termbox) Size() (int, int) {
	termboxMutex.Lock()
	defer termboxMutex.Unlock()
	return termbox.Size()
}
