package peco

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"unicode"

	"github.com/google/btree"
	"github.com/nsf/termbox-go"
	"github.com/peco/peco/internal/keyseq"
)

// ErrUserCanceled is used to signal that the user deliberately
// canceled using peco
var ErrUserCanceled = errors.New("canceled")

// This is the global map of canonical action name to actions
var nameToActions map[string]Action

// This is the default keybinding used by NewKeymap()
var defaultKeyBinding map[string]Action

// Execute fulfills the Action interface for AfterFunc
func (a ActionFunc) Execute(i *Input, e termbox.Event) {
	a(i, e)
}

func (a ActionFunc) registerKeySequence(k keyseq.KeyList) {
	defaultKeyBinding[k.String()] = a
}

// Register fulfills the Action interface for AfterFunc. Registers `a`
// into the global action registry by the name `name`, and maps to
// default keys via `defaultKeys`
func (a ActionFunc) Register(name string, defaultKeys ...termbox.Key) {
	nameToActions["peco."+name] = a
	for _, k := range defaultKeys {
		a.registerKeySequence(keyseq.KeyList{keyseq.NewKeyFromKey(k)})
	}
}

// RegisterKeySequence satisfies the Action interface for AfterFunc.
// Registers the action to be mapped against a key sequence
func (a ActionFunc) RegisterKeySequence(name string, k keyseq.KeyList) {
	nameToActions["peco."+name] = a
	a.registerKeySequence(k)
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
	ActionFunc(doRotateFilter).Register("RotateFilter", termbox.KeyCtrlR)
	wrapDeprecated(doRotateFilter, "RotateMatcher", "RotateFilter").Register("RotateMatcher")
	ActionFunc(doBackToInitialFilter).Register("BackToInitialFilter")

	ActionFunc(doSelectUp).Register("SelectUp", termbox.KeyArrowUp, termbox.KeyCtrlP)
	wrapDeprecated(doSelectDown, "SelectNext", "SelectUp/SelectDown").Register("SelectNext")

	ActionFunc(doScrollPageDown).Register("ScrollPageDown", termbox.KeyArrowRight)
	wrapDeprecated(doScrollPageDown, "SelectNextPage", "ScrollPageDown/ScrollPageUp").Register("SelectNextPage")

	ActionFunc(doSelectDown).Register("SelectDown", termbox.KeyArrowDown, termbox.KeyCtrlN)
	wrapDeprecated(doSelectUp, "SelectPrevious", "SelectUp/SelectDown").Register("SelectPrevious")

	ActionFunc(doScrollPageUp).Register("ScrollPageUp", termbox.KeyArrowLeft)
	wrapDeprecated(doScrollPageUp, "SelectPreviousPage", "ScrollPageDown/ScrollPageUp").Register("SelectPreviousPage")

	ActionFunc(doScrollLeft).Register("ScrollLeft")
	ActionFunc(doScrollRight).Register("ScrollRight")

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
	wrapDeprecated(doToggleRangeMode, "ToggleSelectMode", "ToggleRangeMode").Register("ToggleSelectMode")
	wrapDeprecated(doCancelRangeMode, "CancelSelectMode", "CancelRangeMode").Register("CancelSelectMode")
	ActionFunc(doToggleRangeMode).Register("ToggleRangeMode")
	ActionFunc(doCancelRangeMode).Register("CancelRangeMode")
	ActionFunc(doToggleQuery).Register("ToggleQuery", termbox.KeyCtrlT)
	ActionFunc(doRefreshScreen).Register("RefreshScreen", termbox.KeyCtrlL)
	ActionFunc(doToggleSingleKeyJump).Register("ToggleSingleKeyJump")

	ActionFunc(doKonamiCommand).RegisterKeySequence(
		"KonamiCommand",
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

	ch := ev.Ch
	if ch <= 0 {
		return
	}

	if i.IsSingleKeyJumpMode() {
		doSingleKeyJump(i, ev)
		return
	}

	if i.QueryLen() == i.CaretPos() {
		i.AppendQuery(ch)
	} else {
		i.InsertQueryAt(ch, i.CaretPos())
	}
	i.MoveCaretPos(1)
	i.DrawPrompt() // Update prompt before running query

	i.ExecQuery()
}

func doRotateFilter(i *Input, ev termbox.Event) {
	trace("doRotateFitler: START")
	defer trace("doRotateFitler: END")
	i.RotateFilter()
	if i.ExecQuery() {
		return
	}
	i.SendDrawPrompt()
}

func doBackToInitialFilter(i *Input, ev termbox.Event) {
	trace("doBackToInitialFilter: START")
	defer trace("doBackToInitialFilter: END")
	i.ResetSelectedFilter()
	if i.ExecQuery() {
		return
	}
	i.SendDrawPrompt()
}

func doToggleSelection(i *Input, _ termbox.Event) {
	l, err := i.GetCurrentLineBuffer().LineAt(i.currentLine)
	if err != nil {
		return
	}

	if i.selection.Has(l) {
		i.selection.Remove(l)
		return
	}
	i.selection.Add(l)
}

func doToggleRangeMode(i *Input, _ termbox.Event) {
	trace("doToggleRangeMode: START")
	defer trace("doToggleRangeMode: END")
	if i.IsRangeMode() {
		i.selectionRangeStart = invalidSelectionRange
	} else {
		i.selectionRangeStart = i.currentLine
		i.SelectionAdd(i.currentLine)
	}
}

func doCancelRangeMode(i *Input, _ termbox.Event) {
	i.selectionRangeStart = invalidSelectionRange
}

func doSelectNone(i *Input, _ termbox.Event) {
	i.SelectionClear()
}

func doSelectAll(i *Input, _ termbox.Event) {
	b := i.GetCurrentLineBuffer()
	for x := 0; x < b.Size(); x++ {
		if l, err := b.LineAt(x); err == nil {
			l.SetDirty(true)
			i.selection.Add(l)
		} else {
			i.selection.Remove(l)
		}
	}
	i.SendDraw(false)
}

func doSelectVisible(i *Input, _ termbox.Event) {
	trace("doSelectVisible: START")
	defer trace("doSelectVisible: END")
	b := i.GetCurrentLineBuffer()
	pc := PageCrop{i.currentPage.perPage, i.currentPage.page}
	lb := pc.Crop(b)
	for x := 0; x < lb.Size(); x++ {
		l, err := lb.LineAt(x)
		if err != nil {
			continue
		}
		l.SetDirty(true)
		i.selection.Add(l)
	}
	i.SendDraw(false)
}

func doFinish(i *Input, _ termbox.Event) {
	trace("doFinish: START")
	defer trace("doFinish: END")

	// Must end with all the selected lines.
	if i.SelectionLen() == 0 {
		i.SelectionAdd(i.currentLine)
	}

	i.resultCh = make(chan Line)
	go func() {
		i.selection.Ascend(func(it btree.Item) bool {
			i.resultCh <- it.(Line)
			return true
		})
		close(i.resultCh)
	}()

	i.ExitWith(nil)
}

func doCancel(i *Input, ev termbox.Event) {
	if i.keymap.seq.InMiddleOfChain() {
		i.keymap.seq.CancelChain()
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
	trace("doSelectDown: START")
	defer trace("doSelectDown: END")
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

func doScrollLeft(i *Input, ev termbox.Event) {
	i.SendPaging(ToScrollLeft)
}

func doScrollRight(i *Input, ev termbox.Event) {
	i.SendPaging(ToScrollRight)
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
	trace("doInvertSelection: START")
	defer trace("doInvertSelection: END")

	old := i.selection
	i.SelectionClear()
	b := i.GetCurrentLineBuffer()

	for x := 0; x < b.Size(); x++ {
		if l, err := b.LineAt(x); err == nil {
			l.SetDirty(true)
			i.selection.Add(l)
		} else {
			i.selection.Remove(l)
		}
	}

	old.Ascend(func(it btree.Item) bool {
		i.selection.Delete(it.(Line))
		return true
	})

	i.SendDraw(false)
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
	i.DrawPrompt()
}

func doForwardWord(i *Input, _ termbox.Event) {
	if i.CaretPos() >= i.QueryLen() {
		return
	}
	defer i.DrawPrompt()

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
	defer i.DrawPrompt()

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
	i.DrawPrompt()
}

func doBackwardChar(i *Input, _ termbox.Event) {
	if i.CaretPos() <= 0 {
		return
	}
	i.MoveCaretPos(-1)
	i.DrawPrompt()
}

func doDeleteForwardWord(i *Input, _ termbox.Event) {
	if i.QueryLen() <= i.CaretPos() {
		return
	}
	defer i.DrawPrompt()

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
}

func doBeginningOfLine(i *Input, _ termbox.Event) {
	i.SetCaretPos(0)
	i.DrawPrompt()
}

func doEndOfLine(i *Input, _ termbox.Event) {
	i.SetCaretPos(i.QueryLen())
	i.DrawPrompt()
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
	i.DrawPrompt()
}

func doKillEndOfLine(i *Input, _ termbox.Event) {
	if i.QueryLen() <= i.CaretPos() {
		return
	}

	i.SetQuery(i.Query()[0:i.CaretPos()])
	if i.ExecQuery() {
		return
	}
	i.DrawPrompt()
}

func doDeleteAll(i *Input, _ termbox.Event) {
	i.SetQuery(make([]rune, 0))
	i.ExecQuery()
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
	i.DrawPrompt()
}

func doDeleteBackwardChar(i *Input, ev termbox.Event) {
	trace("doDeleteBackwardChar: START")
	defer trace("doDeleteBackwardChar: END")

	qlen := i.QueryLen()
	if qlen <= 0 {
		trace("doDeleteBackwardChar: QueryLen <= 0, do nothing")
		return
	}

	pos := i.CaretPos()
	if pos == 0 {
		trace("doDeleteBackwardChar: Already at position 0")
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

	i.DrawPrompt()
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
	i.DrawPrompt()
}

func doKonamiCommand(i *Input, ev termbox.Event) {
	i.SendStatusMsg("All your filters are belongs to us")
}

func doToggleSingleKeyJump(i *Input, ev termbox.Event) {
	trace("Toggling SingleKeyJump")
	i.ToggleSingleKeyJumpMode()
}

func doSingleKeyJump(i *Input, ev termbox.Event) {
	trace("Doing single key jump for %c", ev.Ch)
	index, ok := i.config.SingleKeyJump.PrefixMap[ev.Ch]
	if !ok {
		// Couldn't find it? Do nothing
		return
	}

	i.Batch(func() {
		i.SendPaging(JumpToLineRequest(index))
		doFinish(i, ev)
	})
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

func makeCommandAction(cc *CommandConfig) ActionFunc {
	return func(i *Input, _ termbox.Event) {
		if i.SelectionLen() == 0 {
			i.SelectionAdd(i.currentLine)
		}

		i.selection.Ascend(func(it btree.Item) bool {
			line := it.(Line)

			var f *os.File
			var err error

			args := append([]string{}, cc.Args...)
			for i, v := range args {
				switch v {
				case "$FILE":
					if f == nil {
						f, err = ioutil.TempFile("", "peco")
						if err != nil {
							return false
						}
						f.WriteString(line.Buffer())
						f.Close()
					}
					args[i] = f.Name()
				case "$LINE":
					args[i] = line.Buffer()
				}
			}
			i.SendStatusMsg("Executing " + cc.Name)
			cmd := exec.Command(args[0], args[1:]...)
			if cc.Spawn {
				err = cmd.Start()
				go func() {
					cmd.Wait()
					if f != nil {
						os.Remove(f.Name())
					}
				}()
			} else {
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err = cmd.Run()
				if f != nil {
					os.Remove(f.Name())
				}
				i.ExecQuery()
			}
			if err != nil {
				return false
			}

			return true
		})
	}
}
