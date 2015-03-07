package peco

import (
	"sync"
	"time"

	"github.com/nsf/termbox-go"
)

// Input handles input events from termbox.
type Input struct {
	*Ctx
	mutex         sync.Locker // Currently only used for protecting Alt/Esc workaround
	mod           *time.Timer
	keymap        Keymap
	currentKeySeq []string
}

// Loop watches for incoming events from termbox, and pass them
// to the appropriate handler when something arrives.
func (i *Input) Loop() {
	tracer.Printf("Input.Loop: START")
	defer tracer.Printf("Input.Loop: END")
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
		i.SendDraw(nil)
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
				tracer.Printf("Input.handleInputEvent: Firing delayed input event")
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
			tracer.Printf("Input.handleInputEvent: Firing event")
			i.handleKeyEvent(ev)
		}
	}
}

func (i *Input) handleKeyEvent(ev termbox.Event) {
	tracer.Printf("Input.handleKeyEvent: START")
	defer tracer.Printf("Input.handleKeyEvent: END")
	if h := i.keymap.Handler(ev); h != nil {
		tracer.Printf("Input.handleKeyEvent: Event %#v maps to %s, firing action", ev, h)
		h.Execute(i, ev)
		return
	}
}
