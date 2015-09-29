package peco

import (
	"time"

	"github.com/nsf/termbox-go"
)

// Loop watches for incoming events from termbox, and pass them
// to the appropriate handler when something arrives.
func (i *Input) Loop() {
	trace("Input.Loop: START")
	defer trace("Input.Loop: END")
	defer i.ReleaseWaitGroup()

	evCh := screen.PollEvent()

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
		i.SendDraw(false)
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
				trace("Input.handleInputEvent: Firing delayed input event")
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
			trace("Input.handleInputEvent: Firing event")
			i.handleKeyEvent(ev)
		}
	}
}

func (i *Input) handleKeyEvent(ev termbox.Event) {
	trace("Input.handleKeyEvent: START")
	defer trace("Input.handleKeyEvent: END")
	if a := i.keymap.LookupAction(ev); a != nil {
		trace("Input.handleKeyEvent: Event %#v maps to %s, firing action", ev, a)
		a.Execute(i, ev)
		return
	}
}
