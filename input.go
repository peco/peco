package peco

import "github.com/nsf/termbox-go"

type Input struct {
	*Ctx
}

func (i *Input) Loop() {
	defer i.ReleaseWaitGroup()

	for {
		select {
		case <-i.LoopCh(): // can only fall here if we closed c.loopCh
			return
		default:
			ev := termbox.PollEvent()
			switch ev.Type {
			case termbox.EventError:
				//update = false
			case termbox.EventResize:
				i.DrawMatches(nil)
			case termbox.EventKey:
				i.handleKeyEvent(ev)
			}
		}
	}
}

func (i *Input) handleKeyEvent(ev termbox.Event) {
	if h := i.config.Keymap.Handler(ev.Key); h != nil {
		h(i, ev)
		return
	}
}
