package peco

import "github.com/nsf/termbox-go"
import "time"

type Input struct {
	*Ctx
}

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

	var mod *time.Timer
	for {
		select {
		case <-i.LoopCh(): // can only fall here if we closed c.loopCh
			return
		case ev := <-evCh:
			switch ev.Type {
			case termbox.EventError:
				//update = false
			case termbox.EventResize:
				i.DrawMatches(nil)
			case termbox.EventKey:
				// ModAl is sequenced letters leading \x1b (i.e. it's Esc).
				// So must wait for a while until key events.
				// If never keys are typed, it should be Esc.
				if ev.Ch == 0 && ev.Key == 27 && mod == nil {
					tmp := ev
					mod = time.AfterFunc(500 * time.Millisecond, func() {
						mod = nil
						i.handleKeyEvent(tmp)
					})
				} else {
					if mod != nil {
						mod.Stop()
						mod = nil
						ev.Mod |= ModAlt
					}
					i.handleKeyEvent(ev)
				}
			}
		}
	}
}

func (i *Input) handleKeyEvent(ev termbox.Event) {
	if h := i.config.Keymap.Handler(ev); h != nil {
		h(i, ev)
		return
	}
}
