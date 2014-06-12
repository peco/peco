package peco

import (
	"unicode"

	"github.com/nsf/termbox-go"
)

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
	if h := i.config.Keymap.Handler(ev.Key); h != nil {
		h(i, ev)
		return
	}
}

func handleAcceptChar(i *Input, ev termbox.Event) {
	if ev.Key == termbox.KeySpace {
		ev.Ch = ' '
	}

	if ev.Ch > 0 {
		if len(i.query) == i.caretPos {
			i.query = append(i.query, ev.Ch)
		} else {
			buf := make([]rune, len(i.query)+1)
			copy(buf, i.query[:i.caretPos])
			buf[i.caretPos] = ev.Ch
			copy(buf[i.caretPos+1:], i.query[i.caretPos:])
			i.query = buf
		}
		i.caretPos++
		i.ExecQuery(string(i.query))
	}
}

// peco.Finish -> end program, exit with success
func handleFinish(i *Input, _ termbox.Event) {
	if len(i.current) == 1 {
		i.result = i.current[0].line
	} else if i.selectedLine > 0 && i.selectedLine < len(i.current) {
		i.result = i.current[i.selectedLine-1].line
	}
	i.Finish()
}

// peco.Cancel -> end program, exit with failure
func handleCancel(i *Input, ev termbox.Event) {
	i.ExitStatus = 1
	i.Finish()
}

func handleSelectPrevious(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToPrevLine
	i.DrawMatches(nil)
}

func handleSelectNext(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToNextLine
	i.DrawMatches(nil)
}

func handleSelectPreviousPage(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToPrevPage
	i.DrawMatches(nil)
}

func handleSelectNextPage(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToNextPage
	i.DrawMatches(nil)
}

func handleForwardWord(i *Input, _ termbox.Event) {
	if i.caretPos >= len(i.query) {
		return
	}

	foundSpace := false
	for pos := i.caretPos; pos < len(i.query); pos++ {
		r := i.query[pos]
		if foundSpace {
			if !unicode.IsSpace(r) {
				i.caretPos = pos
				i.DrawMatches(nil)
				return
			}
		} else {
			if unicode.IsSpace(r) {
				foundSpace = true
			}
		}
	}

	// not found. just move to the end of the buffer
	i.caretPos = len(i.query)
	i.DrawMatches(nil)

}

func handleBackwardWord(i *Input, _ termbox.Event) {
	if i.caretPos == 0 {
		return
	}

	if i.caretPos >= len(i.query) {
		i.caretPos--
	}

	// if we start from a whitespace-ish position, we should
	// rewind to the end of the previous word, and then do the
	// search all over again
SEARCH_PREV_WORD:
	if unicode.IsSpace(i.query[i.caretPos]) {
		for pos := i.caretPos; pos > 0; pos-- {
			if !unicode.IsSpace(i.query[pos]) {
				i.caretPos = pos
				break
			}
		}
	}

	// if we start from the first character of a word, we
	// should attempt to move back and search for the previous word
	if i.caretPos > 0 && unicode.IsSpace(i.query[i.caretPos-1]) {
		i.caretPos--
		goto SEARCH_PREV_WORD
	}

	// Now look for a space
	for pos := i.caretPos; pos > 0; pos-- {
		if unicode.IsSpace(i.query[pos]) {
			i.caretPos = pos + 1
			i.DrawMatches(nil)
			return
		}
	}

	// not found. just move to the beginning of the buffer
	i.caretPos = 0
	i.DrawMatches(nil)
}

func handleForwardChar(i *Input, _ termbox.Event) {
	if i.caretPos >= len(i.query) {
		return
	}
	i.caretPos++
	i.DrawMatches(nil)
}

func handleBackwardChar(i *Input, _ termbox.Event) {
	if i.caretPos <= 0 {
		return
	}
	i.caretPos--
	i.DrawMatches(nil)
}

func handleBeginningOfLine(i *Input, _ termbox.Event) {
	i.caretPos = 0
	i.DrawMatches(nil)
}

func handleEndOfLine(i *Input, _ termbox.Event) {
	i.caretPos = len(i.query)
	i.DrawMatches(nil)
}

func handleKillEndOfLine(i *Input, _ termbox.Event) {
	if len(i.query) <= i.caretPos {
		return
	}

	i.query = i.query[0:i.caretPos]
	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}
	i.DrawMatches(nil)
}

func handleDeleteForwardChar(i *Input, _ termbox.Event) {
	if len(i.query) <= i.caretPos {
		return
	}

	buf := make([]rune, len(i.query)-1)
	copy(buf, i.query[:i.caretPos])
	copy(buf[i.caretPos:], i.query[i.caretPos+1:])
	i.query = buf
	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func handleDeleteBackwardChar(i *Input, ev termbox.Event) {
	if len(i.query) <= 0 {
		return
	}

	switch i.caretPos {
	case 0:
		// No op
		return
	case len(i.query):
		i.query = i.query[:len(i.query)-1]
	default:
		buf := make([]rune, len(i.query)-1)
		copy(buf, i.query[:i.caretPos])
		copy(buf[i.caretPos-1:], i.query[i.caretPos:])
		i.query = buf
	}
	i.caretPos--
	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func handleDeleteForwardWord(i *Input, _ termbox.Event) {
	if len(i.query) <= i.caretPos {
		return
	}

	for pos := i.caretPos; pos < len(i.query); pos++ {
		if pos == len(i.query)-1 {
			i.query = i.query[:i.caretPos]
			break
		}

		if unicode.IsSpace(i.query[pos]) {
			buf := make([]rune, len(i.query)-(pos-i.caretPos))
			copy(buf, i.query[:i.caretPos])
			copy(buf[i.caretPos:], i.query[pos:])
			i.query = buf
			break
		}
	}

	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func handleDeleteBackwardWord(i *Input, _ termbox.Event) {
	if i.caretPos == 0 {
		return
	}

	for pos := i.caretPos - 1; pos >= 0; pos-- {
		if pos == 0 {
			i.query = i.query[i.caretPos:]
			break
		}

		if unicode.IsSpace(i.query[pos]) {
			buf := make([]rune, len(i.query)-(i.caretPos-pos))
			copy(buf, i.query[:pos])
			copy(buf[pos:], i.query[i.caretPos:])
			i.query = buf
			i.caretPos = pos
			break
		}
	}

	if len(i.query) > 0 {
		i.ExecQuery(string(i.query))
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}
