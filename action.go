package peco

import (
	"errors"
	"fmt"
	"unicode"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/keyseq"
)

var ErrUserCanceled = errors.New("canceled")

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

func wrapDeprecated(fn func(*Input, termbox.Event), oldName, newName string) ActionFunc {
	return ActionFunc(func(i *Input, e termbox.Event) {
		i.SendStatusMsg(fmt.Sprintf("%s is deprecated. Use %s", oldName, newName))
		fn(i, e)
	})
}

func init() {
	// Build the global maps
	nameToActions = map[string]Action{}
	defaultKeyBinding = map[string]Action{}

	ActionFunc(doInvertSelection).Register("InvertSelection")
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
	ActionFunc(wrapDeprecated(doSelectDown, "SelectNext", "SelectUp/SelectDown")).Register("SelectNext")

	ActionFunc(doScrollPageDown).Register("ScrollPageDown", termbox.KeyArrowRight)
	ActionFunc(wrapDeprecated(doScrollPageDown, "SelectNextPage", "ScrollPageDown/ScrollPageUp")).Register("SelectNextPage")

	ActionFunc(doSelectDown).Register("SelectDown", termbox.KeyArrowDown, termbox.KeyCtrlN)
	ActionFunc(wrapDeprecated(doSelectUp, "SelectPrevious", "SelectUp/SelectDown")).Register("SelectPrevious")

	ActionFunc(doScrollPageUp).Register("ScrollPageUp", termbox.KeyArrowLeft)
	ActionFunc(wrapDeprecated(doScrollPageUp, "SelectPreviousPage", "ScrollPageDown/ScrollPageUp")).Register("SelectPreviousPage")

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
	ActionFunc(wrapDeprecated(doToggleRangeMode, "ToggleSelectMode", "ToggleRangeMode")).Register("ToggleSelectMode")
	ActionFunc(wrapDeprecated(doCancelRangeMode, "CancelSelectMode", "CancelRangeMode")).Register("CancelSelectMode")
	ActionFunc(doToggleRangeMode).Register("ToggleRangeMode")
	ActionFunc(doCancelRangeMode).Register("CancelRangeMode")
	ActionFunc(doToggleQuery).Register("ToggleQuery", termbox.KeyCtrlT)
	ActionFunc(doRefreshScreen).Register("RefreshScreen", termbox.KeyCtrlL)

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

	if ch := ev.Ch; ch > 0 {
		if i.QueryLen() == i.CaretPos() {
			i.AppendQuery(ch)
		} else {
			i.InsertQueryAt(ch, i.CaretPos())
		}
		i.MoveCaretPos(1)
		i.DrawPrompt() // Update prompt before running query
		i.ExecQuery()
	}
}

func doRotateMatcher(i *Input, ev termbox.Event) {
	i.RotateFilter()
	if i.ExecQuery() {
		return
	}
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
		i.selectionRangeStart = invalidSelectionRange
	} else {
		i.selectionRangeStart = i.currentLine
		i.SelectionAdd(i.currentPage.offset + i.currentLine)
	}
}

func doCancelRangeMode(i *Input, _ termbox.Event) {
	i.selectionRangeStart = invalidSelectionRange
}

func doSelectNone(i *Input, _ termbox.Event) {
	i.selection.Clear()
}

func doSelectAll(i *Input, _ termbox.Event) {
	for lineno := 1; lineno <= len(i.current); lineno++ {
		i.selection.Add(lineno)
	}
}

func doSelectVisible(i *Input, _ termbox.Event) {
	pageStart := i.currentPage.offset
	pageEnd := pageStart + i.currentPage.perPage
	for lineno := pageStart; lineno <= pageEnd; lineno++ {
		i.selection.Add(lineno)
	}
}

func doFinish(i *Input, _ termbox.Event) {
	tracer.Printf("doFinish: START")
	defer tracer.Printf("doFinish: END")

	// Must end with all the selected lines.
	if i.SelectionLen() == 0 {
		i.SelectionAdd(i.currentLine)
	}

	i.resultCh = make(chan Line)
	go func() {
		buf := i.GetCurrentLineBuffer()
		max := buf.Size()
		for x := 0; x < max; x++ {
			if i.selection.Has(x + 1) {
				if l, err := buf.LineAt(x); err == nil {
					i.resultCh <- l
				}
			}
		}
		close(i.resultCh)
	}()

	i.ExitWith(nil)
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
	i.ExitWith(ErrUserCanceled)
}

func doSelectDown(i *Input, ev termbox.Event) {
	tracer.Printf("doSelectDown: START")
	defer tracer.Printf("doSelectDown: END")
	i.SendPaging(ToLineBelow)
}

func doSelectUp(i *Input, ev termbox.Event) {
	i.SendPaging(ToLineAbove)
}

func doScrollPageUp(i *Input, ev termbox.Event) {
	i.SendPaging(ToScrollPageUp)
}

func doScrollPageDown(i *Input, ev termbox.Event) {
	i.SendPaging(ToScrollPageDown)
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

func doInvertSelection(i *Input, _ termbox.Event) {
	i.selection.Invert(i.GetCurrentLen())
}

func doDeleteBackwardWord(i *Input, _ termbox.Event) {
	if i.CaretPos() == 0 {
		return
	}

	q := i.Query()
	start := i.CaretPos()
	if l := len(q); l <= start {
		start = l
	}

	sepFunc := unicode.IsSpace
	if unicode.IsSpace(q[start-1]) {
		sepFunc = func(r rune) bool { return !unicode.IsSpace(r) }
	}

	found := false
	for pos := start - 1; pos >= 0; pos-- {
		if sepFunc(q[pos]) {
			buf := make([]rune, len(q)-(start-pos-1))
			copy(buf, q[:pos+1])
			copy(buf[pos+1:], q[start:])
			i.SetQuery(buf)
			i.SetCaretPos(pos + 1)
			found = true
			break
		}
	}

	if !found {
		i.SetQuery(q[start:])
		i.SetCaretPos(0)
	}
	if i.ExecQuery() {
		return
	}

	i.SetCurrent(nil)
}

func doForwardWord(i *Input, _ termbox.Event) {
	if i.CaretPos() >= i.QueryLen() {
		return
	}

	foundSpace := false
	for pos := i.CaretPos(); pos < i.QueryLen(); pos++ {
		r := i.Query()[pos]
		if foundSpace {
			if !unicode.IsSpace(r) {
				i.SetCaretPos(pos)
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

}

func doBackwardWord(i *Input, _ termbox.Event) {
	if i.CaretPos() == 0 {
		return
	}

	if i.CaretPos() >= i.QueryLen() {
		i.MoveCaretPos(-1)
	}

	// if we start from a whitespace-ish position, we should
	// rewind to the end of the previous word, and then do the
	// search all over again
SEARCH_PREV_WORD:
	if unicode.IsSpace(i.Query()[i.CaretPos()]) {
		for pos := i.CaretPos(); pos > 0; pos-- {
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
			return
		}
	}

	// not found. just move to the beginning of the buffer
	i.SetCaretPos(0)
}

func doForwardChar(i *Input, _ termbox.Event) {
	if i.CaretPos() >= i.QueryLen() {
		return
	}
	i.MoveCaretPos(1)
}

func doBackwardChar(i *Input, _ termbox.Event) {
	if i.CaretPos() <= 0 {
		return
	}
	i.MoveCaretPos(-1)
	i.SendDraw(nil)
}

func doDeleteForwardWord(i *Input, _ termbox.Event) {
	if i.QueryLen() <= i.CaretPos() {
		return
	}

	start := i.CaretPos()

	// If we are on a word (non-Space, delete till the end of the word.
	// If we are on a space, delete till the end of space.

	q := i.Query()
	sepFunc := unicode.IsSpace
	if unicode.IsSpace(q[start]) {
		sepFunc = func(r rune) bool { return !unicode.IsSpace(r) }
	}

	for pos := start; pos < i.QueryLen(); pos++ {
		if pos == i.QueryLen()-1 {
			i.SetQuery(q[:start])
			i.SetCaretPos(start)
			break
		}

		if sepFunc(q[pos]) {
			buf := make([]rune, i.QueryLen()-(pos-start))
			copy(buf, q[:start])
			copy(buf[start:], q[pos:])
			i.SetQuery(buf)
			i.SetCaretPos(start)
			break
		}
	}

	if i.ExecQuery() {
		return
	}

	i.SetCurrent(nil)
}

func doBeginningOfLine(i *Input, _ termbox.Event) {
	i.SetCaretPos(0)
}

func doEndOfLine(i *Input, _ termbox.Event) {
	i.SetCaretPos(i.QueryLen())
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
	i.SetCurrent(nil)
}

func doKillEndOfLine(i *Input, _ termbox.Event) {
	if i.QueryLen() <= i.CaretPos() {
		return
	}

	i.SetQuery(i.Query()[0:i.CaretPos()])
	if i.ExecQuery() {
		return
	}
	i.SetCurrent(nil)
}

func doDeleteAll(i *Input, _ termbox.Event) {
	i.SetQuery(make([]rune, 0))
	i.SetCurrent(nil)
}

func doDeleteForwardChar(i *Input, _ termbox.Event) {
	if i.QueryLen() <= i.CaretPos() {
		return
	}

	pos := i.CaretPos()
	buf := make([]rune, i.QueryLen()-1)
	copy(buf, i.Query()[:i.CaretPos()])
	copy(buf[i.CaretPos():], i.Query()[i.CaretPos()+1:])
	i.SetQuery(buf)
	i.SetCaretPos(pos)

	if i.ExecQuery() {
		return
	}

	i.SetCurrent(nil)
}

func doDeleteBackwardChar(i *Input, ev termbox.Event) {
	tracer.Printf("doDeleteBackwardChar: START")
	defer tracer.Printf("doDeleteBackwardChar: END")

	qlen := i.QueryLen()
	if qlen <= 0 {
		tracer.Printf("doDeleteBackwardChar: QueryLen <= 0, do nothing")
		return
	}

	pos := i.CaretPos()
	if pos == 0 {
		tracer.Printf("doDeleteBackwardChar: Already at position 0")
		// No op
		return
	}

	var buf []rune
	if qlen == 1 {
		// Micro optimization
		buf = []rune{}
	} else {
		q := i.Query()
		if pos == qlen {
			buf = q[:qlen-1 : qlen-1]
		} else {
			buf = make([]rune, qlen-1)
			copy(buf, q[:pos])
			copy(buf[pos-1:], q[pos:])
		}
	}
	i.SetQuery(buf)
	i.SetCaretPos(pos - 1)

	if i.ExecQuery() {
		return
	}

	i.SetActiveLineBuffer(i.rawLineBuffer)
}

func doRefreshScreen(i *Input, _ termbox.Event) {
	i.ExecQuery()
}

func doToggleQuery(i *Input, _ termbox.Event) {
	q := i.Query()
	if len(q) == 0 {
		sq := i.SavedQuery()
		if len(sq) == 0 {
			return
		}
		i.SetQuery(sq)
		i.SetSavedQuery([]rune{})
	} else {
		i.SetSavedQuery(q)
		i.SetQuery([]rune{})
	}

	if i.ExecQuery() {
		return
	}
	i.SetCurrent(nil)
}

func doKonamiCommand(i *Input, ev termbox.Event) {
	i.SendStatusMsg("All your filters are belongs to us")
}

func makeCombinedAction(actions ...Action) ActionFunc {
	return ActionFunc(func(i *Input, ev termbox.Event) {
		i.Batch(func() {
			for _, a := range actions {
				a.Execute(i, ev)
			}
		})
	})
}
