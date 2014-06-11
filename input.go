package peco

import "github.com/nsf/termbox-go"

type Input struct {
	*Ctx
}

func (i *Input) Loop() {
	i.AddWaitGroup()
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
	if h := i.config.Keymap[ev.Key]; h != nil {
		h(i)
		return
	}

	if ev.Key == termbox.KeySpace {
		ev.Ch = ' '
	}

	if ev.Ch > 0 {
		i.query = append(i.query, ev.Ch)
		i.ExecQuery(string(i.query))
	}
}
