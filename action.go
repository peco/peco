package peco

import (
	"unicode"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/keyseq"
)

// Action describes an action that can be executed upon receiving user
// input. It's an interface so you can create any kind of Action you need,
// but most everything is implemented in terms of ActionFunc, which is
// callback based Action
type Action interface {
	Register(string, ...termbox.Key)
	RegisterKeySequence(keyseq.KeyList)
	Execute(*Input, termbox.Event)
}

// ActionFunc is a type of Action that is basically just a callback.
type ActionFunc func(*Input, termbox.Event)

// This is the global map of canonical action name to actions
var nameToActions map[string]Action

// This is the default keybinding used by NewKeymap()
var defaultKeyBinding map[string]Action

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
		a.RegisterKeySequence(keyseq.KeyList{keyseq.NewKeyFromKey(k)})
	}
}

// RegisterKeySequence satisfies the Action interface for AfterFun.
// Registers the action to be mapped against a key sequence
func (a ActionFunc) RegisterKeySequence(k keyseq.KeyList) {
	defaultKeyBinding[k.String()] = a
}

func init() {
	// Build the global maps
	nameToActions = map[string]Action{}
	defaultKeyBinding = map[string]Action{}

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
	ActionFunc(doDeleteForwardChar).Register("DeleteForwardChar", termbox.KeyCtrlD)
	ActionFunc(doDeleteForwardWord).Register("DeleteForwardWord")
	ActionFunc(doEndOfFile).Register("EndOfFile")
	ActionFunc(doEndOfLine).Register("EndOfLine", termbox.KeyCtrlE)
	ActionFunc(doFinish).Register("Finish", termbox.KeyEnter)
	ActionFunc(doForwardChar).Register("ForwardChar", termbox.KeyCtrlF)
	ActionFunc(doForwardWord).Register("ForwardWord")
	ActionFunc(doKillEndOfLine).Register("KillEndOfLine", termbox.KeyCtrlK)
	ActionFunc(doKillBeginningOfLine).Register("KillBeginningOfLine", termbox.KeyCtrlU)
	ActionFunc(doRotateMatcher).Register("RotateMatcher", termbox.KeyCtrlR)

	ActionFunc(doSelectUp).Register("SelectUp", termbox.KeyArrowUp, termbox.KeyCtrlP)
	ActionFunc(func(i *Input, ev termbox.Event) {
		i.SendStatusMsg("SelectNext is deprecated. Use SelectUp/SelectDown")
		doSelectUp(i, ev)
	}).Register("SelectNext")

	ActionFunc(doScrollPageDown).Register("ScrollPageDown", termbox.KeyArrowRight)
	ActionFunc(func(i *Input, ev termbox.Event) {
		i.SendStatusMsg("SelectNextPage is deprecated. Use ScrollPageDown/ScrollPageUp")
		doScrollPageDown(i, ev)
	}).Register("SelectNextPage")

	ActionFunc(doSelectDown).Register("SelectDown", termbox.KeyArrowDown, termbox.KeyCtrlN)
	ActionFunc(func(i *Input, ev termbox.Event) {
		i.SendStatusMsg("SelectPrevious is deprecated. Use SelectUp/SelectDown")
		doSelectDown(i, ev)
	}).Register("SelectPrevious")

	ActionFunc(doScrollPageUp).Register("ScrollPageUp", termbox.KeyArrowLeft)
	ActionFunc(func(i *Input, ev termbox.Event) {
		i.SendStatusMsg("SelectPreviousPage is deprecated. Uselect ScrollPageDown/ScrollPageUp")
		doScrollPageUp(i, ev)
	}).Register("SelectPreviousPage")

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
	ActionFunc(func(i *Input, ev termbox.Event) {
		i.SendStatusMsg("ToggleSelectMode is deprecated. Use ToggleRangeMode")
		doToggleRangeMode(i, ev)
	}).Register("ToggleSelectMode")
	ActionFunc(func(i *Input, ev termbox.Event) {
		i.SendStatusMsg("CancelSelectMode is deprecated. Use CancelRangeMode")
		doCancelRangeMode(i, ev)
	}).Register("CancelSelectMode")
	ActionFunc(doToggleRangeMode).Register("ToggleRangeMode")
	ActionFunc(doCancelRangeMode).Register("CancelRangeMode")

	ActionFunc(doKonamiCommand).RegisterKeySequence(
		keyseq.KeyList{
			keyseq.Key{Modifier: 0, Key: termbox.KeyCtrlX, Ch: 0},
			keyseq.Key{Modifier: 0, Key: termbox.KeyArrowUp, Ch: 0},
			keyseq.Key{Modifier: 0, Key: termbox.KeyArrowUp, Ch: 0},
			keyseq.Key{Modifier: 0, Key: termbox.KeyArrowDown, Ch: 0},
			keyseq.Key{Modifier: 0, Key: termbox.KeyArrowDown, Ch: 0},
			keyseq.Key{Modifier: 0, Key: termbox.KeyArrowLeft, Ch: 0},
			keyseq.Key{Modifier: 0, Key: termbox.KeyArrowRight, Ch: 0},
			keyseq.Key{Modifier: 0, Key: termbox.KeyArrowLeft, Ch: 0},
			keyseq.Key{Modifier: 0, Key: termbox.KeyArrowRight, Ch: 0},
			keyseq.Key{Modifier: 0, Key: 0, Ch: 'b'},
			keyseq.Key{Modifier: 0, Key: 0, Ch: 'a'},
		},
	)
}

// This is a noop action
func doNothing(_ *Input, _ termbox.Event) {}

// This is an exception to the rule. This does not get registered
// anywhere. You just call it directly
func doAcceptChar(i *Input, ev termbox.Event) {
	if ev.Key == termbox.KeySpace {
		ev.Ch = ' '
	}

	if ev.Ch > 0 {
		if i.QueryLen() == i.CaretPos().Int() {
			i.AppendQuery(ev.Ch)
		} else {
			i.InsertQueryAt(ev.Ch, i.CaretPos().Int())
		}
		i.MoveCaretPos(1)
		i.ExecQuery()
	}
}

func doRotateMatcher(i *Input, ev termbox.Event) {
	i.RotateMatcher()
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

func doToggleRangeMode(i *Input, _ termbox.Event) {
	if i.IsRangeMode() {
		for _, line := range i.SelectedRange() {
			i.selection.Add(line)
		}
		i.selection.Add(i.currentLine)

		i.selectionRangeStart = invalidSelectionRange
	} else {
		i.selectionRangeStart = i.currentLine
	}
	i.DrawMatches(nil)
}

func doCancelRangeMode(i *Input, _ termbox.Event) {
	i.selectionRangeStart = invalidSelectionRange
	i.DrawMatches(nil)
}

func doSelectNone(i *Input, _ termbox.Event) {
	i.selection.Clear()
	i.DrawMatches(nil)
}

func doSelectAll(i *Input, _ termbox.Event) {
	for lineno := 1; lineno <= len(i.current); lineno++ {
		i.selection.Add(lineno)
	}
	i.DrawMatches(nil)
}

func doSelectVisible(i *Input, _ termbox.Event) {
	pageStart := i.currentPage.offset
	pageEnd := pageStart + i.currentPage.perPage
	for lineno := pageStart; lineno <= pageEnd; lineno++ {
		i.selection.Add(lineno)
	}
	i.DrawMatches(nil)
}

func doFinish(i *Input, _ termbox.Event) {
	// Must end with all the selected lines.
	if i.selection.Len() == 0 {
		i.selection.Add(i.currentLine)
	}

	i.result = []Match{}
	for _, lineno := range append(i.selection, i.SelectedRange()...) {
		if lineno <= len(i.current) {
			i.result = append(i.result, i.current[lineno-1])
		}
	}
	i.ExitWith(0)
}

func doCancel(i *Input, ev termbox.Event) {
	if i.keymap.Keyseq.InMiddleOfChain() {
		i.keymap.Keyseq.CancelChain()
		return
	}

	if i.IsRangeMode() {
		doCancelRangeMode(i, ev)
		return
	}

	// peco.Cancel -> end program, exit with failure
	i.ExitWith(1)
}

func doSelectDown(i *Input, ev termbox.Event) {
	i.SendPaging(ToLineBelow)
	i.DrawMatches(nil)
}

func doSelectUp(i *Input, ev termbox.Event) {
	i.SendPaging(ToLineAbove)
	i.DrawMatches(nil)
}

func doScrollPageUp(i *Input, ev termbox.Event) {
	i.SendPaging(ToScrollPageUp)
	i.DrawMatches(nil)
}

func doScrollPageDown(i *Input, ev termbox.Event) {
	i.SendPaging(ToScrollPageDown)
	i.DrawMatches(nil)
}

func doToggleSelectionAndSelectNext(i *Input, ev termbox.Event) {
	i.Batch(func() {
		doToggleSelection(i, ev)
		// XXX This is sucky. Fix later
		if i.layoutType == "top-down" {
			doSelectDown(i, ev)
		} else {
			doSelectUp(i, ev)
		}
	})
}

func doDeleteBackwardWord(i *Input, _ termbox.Event) {
	if i.CaretPos() == 0 {
		return
	}

	for pos := i.CaretPos().Int() - 1; pos >= 0; pos-- {
		q := i.Query()
		if pos == 0 {
			i.SetQuery(q[i.CaretPos().Int():])
			break
		}

		if unicode.IsSpace(q[pos]) {
			buf := make([]rune, q.QueryLen()-(i.CaretPos().Int()-pos))
			copy(buf, q[:pos])
			copy(buf[pos:], q[i.CaretPos().Int():])
			i.SetQuery(buf)
			i.SetCaretPos(pos)
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
	if i.CaretPos().Int() >= i.QueryLen() {
		return
	}

	foundSpace := false
	for pos := i.CaretPos().Int(); pos < i.QueryLen(); pos++ {
		r := i.Query()[pos]
		if foundSpace {
			if !unicode.IsSpace(r) {
				i.SetCaretPos(pos)
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
	i.SetCaretPos(i.QueryLen())
	i.DrawMatches(nil)

}

func doBackwardWord(i *Input, _ termbox.Event) {
	if i.CaretPos().Int() == 0 {
		return
	}

	if i.CaretPos().Int() >= i.QueryLen() {
		i.MoveCaretPos(-1)
	}

	// if we start from a whitespace-ish position, we should
	// rewind to the end of the previous word, and then do the
	// search all over again
SEARCH_PREV_WORD:
	if unicode.IsSpace(i.Query()[i.CaretPos().Int()]) {
		for pos := i.CaretPos().Int(); pos > 0; pos-- {
			if !unicode.IsSpace(i.Query()[pos]) {
				i.SetCaretPos(pos)
				break
			}
		}
	}

	// if we start from the first character of a word, we
	// should attempt to move back and search for the previous word
	if i.CaretPos() > 0 && unicode.IsSpace(i.Query()[i.CaretPos()-1]) {
		i.MoveCaretPos(-1)
		goto SEARCH_PREV_WORD
	}

	// Now look for a space
	for pos := i.CaretPos(); pos > 0; pos-- {
		if unicode.IsSpace(i.Query()[pos]) {
			i.SetCaretPos(int(pos + 1))
			i.DrawMatches(nil)
			return
		}
	}

	// not found. just move to the beginning of the buffer
	i.SetCaretPos(0)
	i.DrawMatches(nil)
}

func doForwardChar(i *Input, _ termbox.Event) {
	if i.CaretPos().Int() >= i.QueryLen() {
		return
	}
	i.MoveCaretPos(1)
	i.DrawMatches(nil)
}

func doBackwardChar(i *Input, _ termbox.Event) {
	if i.CaretPos() <= 0 {
		return
	}
	i.MoveCaretPos(-1)
	i.DrawMatches(nil)
}

func doDeleteForwardWord(i *Input, _ termbox.Event) {
	if i.QueryLen() <= i.CaretPos().Int() {
		return
	}

	start := i.CaretPos().Int()
	for pos := start; pos < i.QueryLen(); pos++ {
		if pos == i.QueryLen()-1 {
			i.SetQuery(i.Query()[:start])
			i.SetCaretPos(start)
			break
		}

		if unicode.IsSpace(i.Query()[pos]) {
			buf := make([]rune, i.QueryLen()-(pos-start))
			copy(buf, i.Query()[:start])
			copy(buf[start:], i.Query()[pos:])
			i.SetQuery(buf)
			i.SetCaretPos(start)
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
	i.SetCaretPos(0)
	i.DrawMatches(nil)
}

func doEndOfLine(i *Input, _ termbox.Event) {
	i.SetCaretPos(i.QueryLen())
	i.DrawMatches(nil)
}

func doEndOfFile(i *Input, ev termbox.Event) {
	if i.QueryLen() > 0 {
		doDeleteForwardChar(i, ev)
	} else {
		doCancel(i, ev)
	}
}

func doKillBeginningOfLine(i *Input, _ termbox.Event) {
	i.SetQuery(i.Query()[i.CaretPos():])
	i.SetCaretPos(0)
	if i.ExecQuery() {
		return
	}
	i.current = nil
	i.DrawMatches(nil)
}

func doKillEndOfLine(i *Input, _ termbox.Event) {
	if i.QueryLen() <= i.CaretPos().Int() {
		return
	}

	i.SetQuery(i.Query()[0:i.CaretPos()])
	if i.ExecQuery() {
		return
	}
	i.current = nil
	i.DrawMatches(nil)
}

func doDeleteAll(i *Input, _ termbox.Event) {
	i.SetQuery(make([]rune, 0))
	i.current = nil
	i.DrawMatches(nil)
}

func doDeleteForwardChar(i *Input, _ termbox.Event) {
	if i.QueryLen() <= i.CaretPos().Int() {
		return
	}

	pos := i.CaretPos().Int()
	buf := make([]rune, i.QueryLen()-1)
	copy(buf, i.Query()[:i.CaretPos()])
	copy(buf[i.CaretPos():], i.Query()[i.CaretPos()+1:])
	i.SetQuery(buf)
	i.SetCaretPos(pos)

	if i.ExecQuery() {
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func doDeleteBackwardChar(i *Input, ev termbox.Event) {
	if i.QueryLen() <= 0 {
		return
	}

	pos := i.CaretPos().Int()
	switch pos {
	case 0:
		// No op
		return
	case i.QueryLen():
		i.SetQuery(i.Query()[:i.QueryLen()-1])
	default:
		buf := make([]rune, i.QueryLen()-1)
		copy(buf, i.Query()[:i.CaretPos()])
		copy(buf[i.CaretPos()-1:], i.Query()[i.CaretPos():])
		i.SetQuery(buf)
	}
	i.SetCaretPos(pos - 1)

	if i.ExecQuery() {
		return
	}

	i.current = nil
	i.DrawMatches(nil)
}

func doKonamiCommand(i *Input, ev termbox.Event) {
	i.SendStatusMsg("All your filters are blongs to us")
}

func makeCombinedAction(actions ...Action) ActionFunc {
	return func(i *Input, ev termbox.Event) {
		i.Batch(func() {
			for _, a := range actions {
				a.Execute(i, ev)
			}
		})
	}
}
