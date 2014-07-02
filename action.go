package peco

import (
	"unicode"

	"github.com/nsf/termbox-go"
)

var handlers map[string]Action
var defaultKeyBinding map[termbox.Key]Action


type Action interface {
	Execute(*Input, termbox.Event)
}

type ActionFunc func(*Input, termbox.Event)

func (a ActionFunc) Execute(i *Input, e termbox.Event) {
	a(i, e)
}

func (a ActionFunc) register(name string, defaultKeys ...termbox.Key) {
	handlers["peco."+name] = a
	for _, k := range defaultKeys {
		defaultKeyBinding[k] = a
	}
}

func init() {
	handlers = map[string]Action{}
	defaultKeyBinding = map[termbox.Key]Action{}

	ActionFunc(doBeginningOfLine).register("BeginningOfLine", termbox.KeyCtrlA)
	ActionFunc(doBackwardChar).register("BackwardChar", termbox.KeyCtrlB)
	ActionFunc(doBackwardWord).register("BackwardWord")
	ActionFunc(doCancel).register("Cancel", termbox.KeyCtrlC, termbox.KeyEsc)
	ActionFunc(doDeleteAll).register("DeleteAll")
	ActionFunc(doDeleteBackwardChar).register(
		"DeleteBackwardChar",
		termbox.KeyBackspace,
		termbox.KeyBackspace2,
	)
	ActionFunc(doDeleteBackwardWord).register(
		"DeleteBackwardWord",
		termbox.KeyCtrlW,
	)
	ActionFunc(doDeleteForwardChar).register("DeletedForwardChar", termbox.KeyCtrlD)
	ActionFunc(doDeleteForwardWord).register("DeleteForwardWord")
	ActionFunc(doEndOfFile).register("EndOfFile")
	ActionFunc(doEndOfLine).register("EndOfLine", termbox.KeyCtrlE)
	ActionFunc(doFinish).register("Finish", termbox.KeyEnter)
	ActionFunc(doForwardChar).register("ForwardChar", termbox.KeyCtrlF)
	ActionFunc(doForwardWord).register("ForwardWord")
	ActionFunc(doKillEndOfLine).register("KillEndOfLine", termbox.KeyCtrlK)
	ActionFunc(doKillBeginningOfLine).register("KillBeginningOfLine", termbox.KeyCtrlU)
	ActionFunc(doRotateMatcher).register("RotateMatcher", termbox.KeyCtrlR)
	ActionFunc(doSelectNext).register(
		"SelectNext",
		termbox.KeyArrowDown,
		termbox.KeyCtrlN,
	)
	ActionFunc(doSelectNextPage).register(
		"SelectNextPage",
		termbox.KeyArrowRight,
	)
	ActionFunc(doSelectPrevious).register(
		"SelectPrevious",
		termbox.KeyArrowUp,
		termbox.KeyCtrlP,
	)
	ActionFunc(doSelectPreviousPage).register(
		"SelectPreviousPage",
		termbox.KeyArrowLeft,
	)

	ActionFunc(doToggleSelection).register("ToggleSelection")
	ActionFunc(doToggleSelectionAndSelectNext).register(
		"ToggleSelectionAndSelectNext",
		termbox.KeyCtrlSpace,
	)
	ActionFunc(doSelectNone).register(
		"SelectNone",
		termbox.KeyCtrlG,
	)
	ActionFunc(doSelectVisible).register("SelectVisible")
}

func doRotateMatcher(i *Input, ev termbox.Event) {
	i.Ctx.CurrentMatcher++
	if i.Ctx.CurrentMatcher >= len(i.Ctx.Matchers) {
		i.Ctx.CurrentMatcher = 0
	}
	if i.ExecQuery() {
		return
	}
	i.DrawMatches(nil)
}

func doToggleSelection(i *Input, _ termbox.Event) {
	if i.selection.Has(i.currentLine) {
		i.selection.Remove(i.currentLine)
		return
	}
	i.selection.Add(i.currentLine)
}

func doSelectNone(i *Input, _ termbox.Event) {
	i.selection.Clear()
	i.DrawMatches(nil)
}

func doSelectVisible(i *Input, _ termbox.Event) {
	pageStart := i.currentPage.offset
	pageEnd := pageStart + i.currentPage.perPage
	for lineno:=pageStart; lineno <= pageEnd; lineno++ {
		i.selection.Add(lineno)
	}
	i.DrawMatches(nil)
}

func doFinish(i *Input, _ termbox.Event) {
	// Must end with all the selected lines.
	i.selection.Add(i.currentLine)

	i.result = []Match{}
	for _, lineno := range i.selection {
		if lineno <= len(i.current) {
			i.result = append(i.result, i.current[lineno-1])
		}
	}
	i.ExitWith(0)
}

func doCancel(i *Input, ev termbox.Event) {
	// peco.Cancel -> end program, exit with failure
	i.ExitWith(1)
}

func doSelectPrevious(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToPrevLine
	i.DrawMatches(nil)
}

func doSelectNext(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToNextLine
	i.DrawMatches(nil)
}

func doSelectPreviousPage(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToPrevPage
	i.DrawMatches(nil)
}

func doSelectNextPage(i *Input, ev termbox.Event) {
	i.PagingCh() <- ToNextPage
	i.DrawMatches(nil)
}


func doToggleSelectionAndSelectNext(i *Input, ev termbox.Event) {
	doToggleSelection(i, ev)
	doSelectNext(i, ev)
}

func doDeleteBackwardWord(i *Input, _ termbox.Event) {
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

	if i.ExecQuery() {
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func doForwardWord(i *Input, _ termbox.Event) {
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

func doBackwardWord(i *Input, _ termbox.Event) {
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

func doForwardChar(i *Input, _ termbox.Event) {
	if i.caretPos >= len(i.query) {
		return
	}
	i.caretPos++
	i.DrawMatches(nil)
}

func doBackwardChar(i *Input, _ termbox.Event) {
	if i.caretPos <= 0 {
		return
	}
	i.caretPos--
	i.DrawMatches(nil)
}

func doDeleteForwardWord(i *Input, _ termbox.Event) {
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

	if i.ExecQuery() {
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func doBeginningOfLine(i *Input, _ termbox.Event) {
	i.caretPos = 0
	i.DrawMatches(nil)
}

func doEndOfLine(i *Input, _ termbox.Event) {
	i.caretPos = len(i.query)
	i.DrawMatches(nil)
}

func doEndOfFile(i *Input, ev termbox.Event) {
	if len(i.query) > 0 {
		doDeleteForwardChar(i, ev)
	} else {
		doCancel(i, ev)
	}
}

func doKillBeginningOfLine(i *Input, _ termbox.Event) {
	i.query = i.query[i.caretPos:]
	i.caretPos = 0
	if i.ExecQuery() {
		return
	}
	i.current = nil
	i.DrawMatches(nil)
}

func doKillEndOfLine(i *Input, _ termbox.Event) {
	if len(i.query) <= i.caretPos {
		return
	}

	i.query = i.query[0:i.caretPos]
	if i.ExecQuery() {
		return
	}
	i.current = nil
	i.DrawMatches(nil)
}

func doDeleteAll(i *Input, _ termbox.Event) {
	i.query = make([]rune, 0)
	i.current = nil
	i.DrawMatches(nil)
}

func doDeleteForwardChar(i *Input, _ termbox.Event) {
	if len(i.query) <= i.caretPos {
		return
	}

	buf := make([]rune, len(i.query)-1)
	copy(buf, i.query[:i.caretPos])
	copy(buf[i.caretPos:], i.query[i.caretPos+1:])
	i.query = buf

	if i.ExecQuery() {
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func doDeleteBackwardChar(i *Input, ev termbox.Event) {
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

	if i.ExecQuery() {
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}


