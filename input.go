package peco

import (
	"sync"
	"time"

	"github.com/nsf/termbox-go"
)

// Input handles input events from termbox.
type Input struct {
	*Ctx
	mutex  *sync.Mutex // Currently only used for protecting Alt/Esc workaround
	mod    *time.Timer
	keymap Keymap
	currentKeySeq []string
}

// Loop watches for incoming events from termbox, and pass them
// to the appropriate handler when something arrives.
func (i *Input) Loop() {
	defer i.ReleaseWaitGroup()

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
		for {
			evCh <- termbox.PollEvent()
		}
	}()

	for {
		select {
		case <-i.LoopCh(): // can only fall here if we closed c.loopCh
			return
		case ev := <-evCh:
			i.handleInputEvent(ev)
		}
	}
}

func (i *Input) handleInputEvent(ev termbox.Event) {
	switch ev.Type {
	case termbox.EventError:
		//update = false
	case termbox.EventResize:
		i.DrawMatches(nil)
	case termbox.EventKey:
		// ModAlt is a sequence of letters with a leading \x1b (=Esc).
		// It would be nice if termbox differentiated this for us, but
		// we workaround it by waiting (juuuuse a few milliseconds) for
		// extra key events. If no extra events arrive, it should be Esc

		// Smells like Esc or Alt. mod == nil checks for the presense
		// of a previous timer
		if ev.Ch == 0 && ev.Key == 27 && i.mod == nil {
			tmp := ev
			i.mutex.Lock()
			i.mod = time.AfterFunc(50*time.Millisecond, func() {
				i.mutex.Lock()
				i.mod = nil
				i.mutex.Unlock()
				i.handleKeyEvent(tmp)
			})
			i.mutex.Unlock()
		} else {
			// it doesn't look like this is Esc or Alt. If we have a previous
			// timer, stop it because this is probably Alt+ this new key
			i.mutex.Lock()
			if i.mod != nil {
				i.mod.Stop()
				i.mod = nil
				ev.Mod |= termbox.ModAlt
			}
			i.mutex.Unlock()
			i.handleKeyEvent(ev)
		}
	}
}

func (i *Input) handleKeyEvent(ev termbox.Event) {
	if h := i.keymap.Handler(ev); h != nil {
		h.Execute(i, ev)
		return
	}
}
