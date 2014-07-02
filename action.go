package peco

import (
	"unicode"

	"github.com/nsf/termbox-go"
)

// Action describes an action that can be executed upon receiving user
// input. It's an interface so you can create any kind of Action you need,
// but the most everything is implemented in terms of ActionFunc, which is
// callback based Action
type Action interface {
	Register(string, ...termbox.Key)
	Execute(*Input, termbox.Event)
}

// ActionFunc is a type of Action that is basically just a callback.
type ActionFunc func(*Input, termbox.Event)

// This is the global map of canonical action name to actions
var nameToActions map[string]Action

// This is the default keybinding used by NewKeymap()
var defaultKeyBinding map[termbox.Key]Action

// Execute fulfills the Action interface for AfterFunc
func (a ActionFunc) Execute(i *Input, e termbox.Event) {
	a(i, e)
}

// Register fulfills the Actin interface for AfterFunc. Registers `a`
// into the global action registry by the name `name`, and maps to
// default keys via `defaultKeys`
func (a ActionFunc) Register(name string, defaultKeys ...termbox.Key) {
	nameToActions["peco."+name] = a
	for _, k := range defaultKeys {
		defaultKeyBinding[k] = a
	}
}

func init() {
	// Build the global maps
	nameToActions = map[string]Action{}
	defaultKeyBinding = map[termbox.Key]Action{}

	ActionFunc(doBeginningOfLine).Register("BeginningOfLine", termbox.KeyCtrlA)
	ActionFunc(doBackwardChar).Register("BackwardChar", termbox.KeyCtrlB)
	ActionFunc(doBackwardWord).Register("BackwardWord")
	ActionFunc(doCancel).Register("Cancel", termbox.KeyCtrlC, termbox.KeyEsc)
	ActionFunc(doDeleteAll).Register("DeleteAll")
	ActionFunc(doDeleteBackwardChar).Register(
		"DeleteBackwardChar",
		termbox.KeyBackspace,
		termbox.KeyBackspace2,
	)
	ActionFunc(doDeleteBackwardWord).Register(
		"DeleteBackwardWord",
		termbox.KeyCtrlW,
	)
	ActionFunc(doDeleteForwardChar).Register("DeletedForwardChar", termbox.KeyCtrlD)
	ActionFunc(doDeleteForwardWord).Register("DeleteForwardWord")
	ActionFunc(doEndOfFile).Register("EndOfFile")
	ActionFunc(doEndOfLine).Register("EndOfLine", termbox.KeyCtrlE)
	ActionFunc(doFinish).Register("Finish", termbox.KeyEnter)
	ActionFunc(doForwardChar).Register("ForwardChar", termbox.KeyCtrlF)
	ActionFunc(doForwardWord).Register("ForwardWord")
	ActionFunc(doKillEndOfLine).Register("KillEndOfLine", termbox.KeyCtrlK)
	ActionFunc(doKillBeginningOfLine).Register("KillBeginningOfLine", termbox.KeyCtrlU)
	ActionFunc(doRotateMatcher).Register("RotateMatcher", termbox.KeyCtrlR)
	ActionFunc(doSelectNext).Register(
		"SelectNext",
		termbox.KeyArrowDown,
		termbox.KeyCtrlN,
	)
	ActionFunc(doSelectNextPage).Register(
		"SelectNextPage",
		termbox.KeyArrowRight,
	)
	ActionFunc(doSelectPrevious).Register(
		"SelectPrevious",
		termbox.KeyArrowUp,
		termbox.KeyCtrlP,
	)
	ActionFunc(doSelectPreviousPage).Register(
		"SelectPreviousPage",
		termbox.KeyArrowLeft,
	)

	ActionFunc(doToggleSelection).Register("ToggleSelection")
	ActionFunc(doToggleSelectionAndSelectNext).Register(
		"ToggleSelectionAndSelectNext",
		termbox.KeyCtrlSpace,
	)
	ActionFunc(doSelectNone).Register(
		"SelectNone",
		termbox.KeyCtrlG,
	)
	ActionFunc(doSelectAll).Register("SelectAll")
	ActionFunc(doSelectVisible).Register("SelectVisible")
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

func doSelectAll(i *Input, _ termbox.Event) {
	for lineno:=1; lineno <= len(i.current); lineno++ {
		i.selection.Add(lineno)
	}
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


