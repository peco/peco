package percol

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
	switch ev.Key {
	case termbox.KeyEsc:
		close(i.LoopCh())
	case termbox.KeyEnter:
		if len(i.current) == 1 {
			i.result = i.current[0].line
		} else if i.selectedLine > 0 && i.selectedLine < len(i.current) {
			i.result = i.current[i.selectedLine-1].line
		}
		close(i.LoopCh())
	case termbox.KeyArrowUp, termbox.KeyCtrlK:
		if i.selectedLine > 1 { // starts at 1
			i.selectedLine--
			i.DrawMatches(nil)
		}
	case termbox.KeyArrowDown, termbox.KeyCtrlJ:
		i.selectedLine++
		i.DrawMatches(nil)
	case termbox.KeyBackspace, termbox.KeyBackspace2:
		if len(i.query) > 0 {
			i.query = i.query[:len(i.query)-1]
			if len(i.query) > 0 {
				i.ExecQuery(string(i.query))
			} else {
				i.current = nil
				i.DrawMatches(nil)
			}
		}
	default:
		if ev.Key == termbox.KeySpace {
			ev.Ch = ' '
		}

		if ev.Ch > 0 {
			i.query = append(i.query, ev.Ch)
			i.ExecQuery(string(i.query))
		}
	}
}
