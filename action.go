package peco

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strconv"
	"unicode"

	"context"

	"github.com/google/btree"
	"github.com/lestrrat-go/pdebug/v2"
	"github.com/nsf/termbox-go"
	"github.com/peco/peco/buffer"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/ui"
	"github.com/pkg/errors"
)

// This is the global map of canonical action name to actions
var nameToActions map[string]Action

// This is the default keybinding used by NewKeymap()
var defaultKeyBinding map[string]Action

// Execute fulfills the Action interface for AfterFunc
func (a ActionFunc) Execute(ctx context.Context, state *Peco, e termbox.Event) {
	a(ctx, state, e)
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

func wrapDeprecated(fn func(context.Context, *Peco, termbox.Event), oldName, newName string) ActionFunc {
	return ActionFunc(func(ctx context.Context, state *Peco, e termbox.Event) {
		state.Hub().SendStatusMsg(ctx, fmt.Sprintf("%s is deprecated. Use %s", oldName, newName))
		fn(ctx, state, e)
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

	ActionFunc(doScrollFirstItem).Register("ScrollFirstItem", termbox.KeyHome)
	ActionFunc(doScrollLastItem).Register("ScrollLastItem", termbox.KeyEnd)

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

	ActionFunc(doToggleViewArround).Register("ViewArround", termbox.KeyCtrlV)

	ActionFunc(doGoToNextSelection).Register("GoToNextSelection", termbox.KeyCtrlK)
	ActionFunc(doGoToPreviousSelection).Register("	doGoToPreviousSelection", termbox.KeyCtrlJ)

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
func doNothing(_ context.Context, _ *Peco, _ termbox.Event) {}

// This is an exception to the rule. This does not get registered
// anywhere. You just call it directly
func doAcceptChar(ctx context.Context, state *Peco, e termbox.Event) {
	if e.Key == termbox.KeySpace {
		e.Ch = ' '
	}

	ch := e.Ch
	if ch <= 0 {
		return
	}

	if state.SingleKeyJumpMode() {
		doSingleKeyJump(ctx, state, e)
		return
	}

	q := state.Query()
	c := state.Caret()

	q.InsertAt(ch, c.Pos())
	c.Move(1)

	h := state.Hub()
	h.SendDrawPrompt(ctx) // Update prompt before running query

	state.ExecQuery(nil)
}

func doRotateFilter(ctx context.Context, state *Peco, e termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doRotateFilter")
		defer g.End()
	}

	filters := state.Filters()
	filters.Rotate()

	if state.ExecQuery(nil) {
		return
	}
	state.Hub().SendDrawPrompt(ctx)
}

func doBackToInitialFilter(ctx context.Context, state *Peco, e termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doBackToInitialFilter")
		defer g.End()
	}

	filters := state.Filters()
	filters.Reset()

	if state.ExecQuery(nil) {
		return
	}
	state.Hub().SendDrawPrompt(ctx)
}

func doToggleSelection(ctx context.Context, state *Peco, _ termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doToggleSelection")
		defer g.End()
	}

	l, err := state.CurrentLineBuffer().LineAt(state.Location().LineNumber())
	if err != nil {
		return
	}

	selection := state.Selection()
	if selection.Has(l) {
		selection.Remove(l)
		return
	}
	selection.Add(l)
}

func doToggleRangeMode(ctx context.Context, state *Peco, _ termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doToggleRangeMode")
		defer g.End()
	}

	r := state.SelectionRangeStart()
	if r.Valid() {
		r.Reset()
	} else {
		cl := state.Location().LineNumber()
		r.SetValue(cl)
		if l, err := state.CurrentLineBuffer().LineAt(cl); err == nil {
			state.selection.Add(l)
		}
	}
}

func doCancelRangeMode(ctx context.Context, state *Peco, _ termbox.Event) {
	state.SelectionRangeStart().Reset()
}

func doSelectNone(ctx context.Context, state *Peco, _ termbox.Event) {
	state.Selection().Reset()
	state.Hub().SendDraw(ctx, ui.WithLineCache(false))
}

func doSelectAll(ctx context.Context, state *Peco, _ termbox.Event) {
	selection := state.Selection()
	b := state.CurrentLineBuffer()
	for x := 0; x < b.Size(); x++ {
		if l, err := b.LineAt(x); err == nil {
			l.SetDirty(true)
			selection.Add(l)
		} else {
			selection.Remove(l)
		}
	}
	state.Hub().SendDraw(ctx, nil)
}

func doSelectVisible(ctx context.Context, state *Peco, _ termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doSelectVisible")
		defer g.End()
	}

	b := state.CurrentLineBuffer()
	selection := state.Selection()
	lb := buffer.Crop(b, state.Location())
	for x := 0; x < lb.Size(); x++ {
		l, err := lb.LineAt(x)
		if err != nil {
			continue
		}
		l.SetDirty(true)
		selection.Add(l)
	}
	state.Hub().SendDraw(ctx, nil)
}

type errCollectResults struct{}

func (err errCollectResults) Error() string {
	return "collect results"
}
func (err errCollectResults) CollectResults() bool {
	return true
}
func doFinish(ctx context.Context, state *Peco, _ termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doFinish")
		defer g.End()
	}

	ccarg := state.execOnFinish
	if len(ccarg) == 0 {
		state.Exit(ctx, errCollectResults{})
		return
	}

	sel := ui.NewSelection()
	state.Selection().Copy(sel)
	if sel.Len() == 0 {
		if l, err := state.CurrentLineBuffer().LineAt(state.Location().LineNumber()); err == nil {
			sel.Add(l)
		}
	}

	var stdin bytes.Buffer
	sel.Ascend(func(it btree.Item) bool {
		line := it.(line.Line)
		stdin.WriteString(line.Buffer())
		stdin.WriteRune('\n')
		return true
	})

	var err error
	state.Hub().SendStatusMsg(ctx, "Executing "+ccarg)
	cmd := util.Shell(ccarg)
	cmd.Stdin = &stdin
	cmd.Stdout = state.Stdout
	cmd.Stderr = state.Stderr
	// Setup some environment variables. Start with a copy of the current
	// environment...
	env := os.Environ()

	// Add some PECO specific ones...
	// PECO_QUERY: current query value
	// PECO_FILENAME: input file name, if any. "-" for stdin
	// PECO_LINE_COUNT: number of lines in the original input
	// PECO_MATCHED_LINE_COUNT: number of lines matched (number of lines being
	//     sent to stdin of the command being executed)

	if s, ok := state.Source().(*Source); ok {
		env = append(env,
			`PECO_FILENAME=`+s.Name(),
			`PECO_LINE_COUNT=`+strconv.Itoa(s.Size()),
		)
	}

	env = append(env,
		`PECO_QUERY=`+state.Query().String(),
		`PECO_MATCHED_LINE_COUNT=`+strconv.Itoa(sel.Len()),
	)
	cmd.Env = env

	state.screen.Suspend()

	err = cmd.Run()
	state.screen.Resume()
	state.Hub().SendDraw(ctx, ui.WithLineCache(false))
	if err != nil {
		// bail out, or otherwise the user cannot know what happened
		state.Exit(ctx, errors.Wrap(err, `failed to execute command`))
	}
}

func doCancel(ctx context.Context, state *Peco, e termbox.Event) {
	km := state.Keymap()

	if seq := km.Sequence(); seq.InMiddleOfChain() {
		seq.CancelChain()
		return
	}

	if state.SelectionRangeStart().Valid() {
		doCancelRangeMode(ctx, state, e)
		return
	}

	// peco.Cancel -> end program, exit with failure
	err := makeIgnorable(errors.New("user canceled"))
	if state.onCancel == errorKey {
		err = setExitStatus(err, 1)
	}
	state.Exit(ctx, err)
}

func doSelectDown(ctx context.Context, state *Peco, e termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doSelectDown")
		defer g.End()
	}
	state.Hub().SendPaging(ctx, ui.ToLineBelow)
}

func doSelectUp(ctx context.Context, state *Peco, e termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doSelectUp")
		defer g.End()
	}
	state.Hub().SendPaging(ctx, ui.ToLineAbove)
}

func doScrollPageUp(ctx context.Context, state *Peco, e termbox.Event) {
	state.Hub().SendPaging(ctx, ui.ToScrollPageUp)
}

func doScrollPageDown(ctx context.Context, state *Peco, e termbox.Event) {
	state.Hub().SendPaging(ctx, ui.ToScrollPageDown)
}

func doScrollLeft(ctx context.Context, state *Peco, e termbox.Event) {
	state.Hub().SendPaging(ctx, ui.ToScrollLeft)
}

func doScrollRight(ctx context.Context, state *Peco, e termbox.Event) {
	state.Hub().SendPaging(ctx, ui.ToScrollRight)
}

func doScrollFirstItem(ctx context.Context, state *Peco, e termbox.Event) {
	state.Hub().SendPaging(ctx, ui.ToScrollFirstItem)
}

func doScrollLastItem(ctx context.Context, state *Peco, e termbox.Event) {
	state.Hub().SendPaging(ctx, ui.ToScrollLastItem)
}

func doToggleSelectionAndSelectNext(ctx context.Context, state *Peco, e termbox.Event) {
	toplevel, _ := ctx.Value(isTopLevelActionCall).(bool)
	state.Hub().Batch(ctx, func(ctx context.Context) {
		ctx = context.WithValue(ctx, isTopLevelActionCall, false)
		doToggleSelection(ctx, state, e)
		// XXX This is sucky. Fix later
		if state.LayoutType() == "top-down" {
			doSelectDown(ctx, state, e)
		} else {
			doSelectUp(ctx, state, e)
		}
	}, toplevel)
}

func doInvertSelection(ctx context.Context, state *Peco, _ termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doInvertSelection")
		defer g.End()
	}

	selection := state.Selection()
	b := state.CurrentLineBuffer()

	for x := 0; x < b.Size(); x++ {
		if l, err := b.LineAt(x); err == nil {
			l.SetDirty(true)
			if selection.Has(l) {
				selection.Remove(l)
			} else {
				selection.Add(l)
			}
		} else {
			selection.Remove(l)
		}
	}

	state.Hub().SendDraw(ctx, nil)
}

func doDeleteBackwardWord(ctx context.Context, state *Peco, _ termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doDeleteBackwardWord")
		defer g.End()
	}

	c := state.Caret()
	if c.Pos() == 0 {
		return
	}

	q := state.Query()
	pos := q.Len()
	if l := q.Len(); l <= c.Pos() {
		pos = l
	}

	sepFunc := unicode.IsSpace
	if unicode.IsSpace(q.RuneAt(pos - 1)) {
		sepFunc = func(r rune) bool { return !unicode.IsSpace(r) }
	}

	found := false
	start := pos
	for pos = start - 1; pos >= 0; pos-- {
		if sepFunc(q.RuneAt(pos)) {
			q.DeleteRange(pos+1, start)
			c.SetPos(pos + 1)
			found = true
			break
		}
	}

	if !found {
		q.DeleteRange(0, start)
		c.SetPos(0)
	}
	if state.ExecQuery(nil) {
		return
	}
	state.Hub().SendDrawPrompt(ctx)
}

func doForwardWord(ctx context.Context, state *Peco, _ termbox.Event) {
	if state.Caret().Pos() >= state.Query().Len() {
		return
	}
	defer state.Hub().SendDrawPrompt(ctx)

	foundSpace := false
	q := state.Query()
	c := state.Caret()
	for pos := c.Pos(); pos < q.Len(); pos++ {
		r := q.RuneAt(pos)
		if foundSpace {
			if !unicode.IsSpace(r) {
				c.SetPos(pos)
				return
			}
		} else {
			if unicode.IsSpace(r) {
				foundSpace = true
			}
		}
	}

	// not found. just move to the end of the buffer
	c.SetPos(q.Len())
}

func doBackwardWord(ctx context.Context, state *Peco, _ termbox.Event) {
	c := state.Caret()
	q := state.Query()
	if c.Pos() == 0 {
		return
	}
	defer state.Hub().SendDrawPrompt(ctx)

	if c.Pos() >= q.Len() {
		c.Move(-1)
	}

	// if we start from a whitespace-ish position, we should
	// rewind to the end of the previous word, and then do the
	// search all over again
SEARCH_PREV_WORD:
	if unicode.IsSpace(q.RuneAt(c.Pos())) {
		for pos := c.Pos(); pos > 0; pos-- {
			if !unicode.IsSpace(q.RuneAt(pos)) {
				c.SetPos(pos)
				break
			}
		}
	}

	// if we start from the first character of a word, we
	// should attempt to move back and search for the previous word
	if c.Pos() > 0 && unicode.IsSpace(q.RuneAt(c.Pos()-1)) {
		c.Move(-1)
		goto SEARCH_PREV_WORD
	}

	// Now look for a space
	for pos := c.Pos(); pos > 0; pos-- {
		if unicode.IsSpace(q.RuneAt(pos)) {
			c.SetPos(int(pos + 1))
			return
		}
	}

	// not found. just move to the beginning of the buffer
	c.SetPos(0)
}

func doForwardChar(ctx context.Context, state *Peco, _ termbox.Event) {
	c := state.Caret()
	if c.Pos() >= state.Query().Len() {
		return
	}
	c.Move(1)
	state.Hub().SendDrawPrompt(ctx)
}

func doBackwardChar(ctx context.Context, state *Peco, _ termbox.Event) {
	c := state.Caret()
	if c.Pos() <= 0 {
		return
	}
	c.Move(-1)
	state.Hub().SendDrawPrompt(ctx)
}

func doDeleteForwardWord(ctx context.Context, state *Peco, _ termbox.Event) {
	c := state.Caret()
	q := state.Query()
	start := c.Pos()

	if q.Len() <= start {
		return
	}

	// If we are on a word (non-Space, delete till the end of the word.
	// If we are on a space, delete till the end of space.
	sepFunc := unicode.IsSpace
	if unicode.IsSpace(q.RuneAt(start)) {
		sepFunc = func(r rune) bool { return !unicode.IsSpace(r) }
	}

	for pos := start; pos < q.Len(); pos++ {
		if pos == q.Len()-1 {
			q.DeleteRange(start, q.Len())
			c.SetPos(start)
			break
		}

		if sepFunc(q.RuneAt(pos)) {
			q.DeleteRange(start, pos)
			c.SetPos(start)
			break
		}
	}

	if state.ExecQuery(nil) {
		return
	}
	state.Hub().SendDrawPrompt(ctx)
}

func doBeginningOfLine(ctx context.Context, state *Peco, _ termbox.Event) {
	state.Caret().SetPos(0)
	state.Hub().SendDrawPrompt(ctx)
}

func doEndOfLine(ctx context.Context, state *Peco, _ termbox.Event) {
	state.Caret().SetPos(state.Query().Len())
	state.Hub().SendDrawPrompt(ctx)
}

func doEndOfFile(ctx context.Context, state *Peco, e termbox.Event) {
	if state.Query().Len() > 0 {
		doDeleteForwardChar(ctx, state, e)
	} else {
		doCancel(ctx, state, e)
	}
}

func doKillBeginningOfLine(ctx context.Context, state *Peco, _ termbox.Event) {
	q := state.Query()
	q.DeleteRange(0, state.Caret().Pos())
	state.Caret().SetPos(0)
	if state.ExecQuery(nil) {
		return
	}
	state.Hub().SendDrawPrompt(ctx)
}

func doKillEndOfLine(ctx context.Context, state *Peco, _ termbox.Event) {
	if state.Query().Len() <= state.Caret().Pos() {
		return
	}

	q := state.Query()
	q.DeleteRange(state.Caret().Pos(), q.Len())
	if state.ExecQuery(nil) {
		return
	}
	state.Hub().SendDrawPrompt(ctx)
}

func doDeleteAll(ctx context.Context, state *Peco, _ termbox.Event) {
	state.Query().Reset()
	state.ExecQuery(nil)
}

func doDeleteForwardChar(ctx context.Context, state *Peco, _ termbox.Event) {
	q := state.Query()
	c := state.Caret()
	if q.Len() <= c.Pos() {
		return
	}

	pos := c.Pos()
	q.DeleteRange(pos, pos+1)

	if state.ExecQuery(nil) {
		return
	}
	state.Hub().SendDrawPrompt(ctx)
}

func doDeleteBackwardChar(ctx context.Context, state *Peco, e termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doDeleteBackwardChar")
		defer g.End()
	}

	q := state.Query()
	c := state.Caret()
	qlen := q.Len()
	if qlen <= 0 {
		if pdebug.Enabled {
			pdebug.Printf(ctx, "doDeleteBackwardChar: QueryLen <= 0, do nothing")
		}
		return
	}

	pos := c.Pos()
	if pos == 0 {
		if pdebug.Enabled {
			pdebug.Printf(ctx, "doDeleteBackwardChar: Already at position 0")
		}
		// No op
		return
	}

	if qlen == 1 {
		// Micro optimization
		q.Reset()
	} else {
		q.DeleteRange(pos-1, pos)
	}
	c.SetPos(pos - 1)

	if state.ExecQuery(nil) {
		return
	}

	state.Hub().SendDrawPrompt(ctx)
}

func doRefreshScreen(ctx context.Context, state *Peco, _ termbox.Event) {
	state.Hub().SendDraw(ctx, ui.WithLineCache(true))
}

func doToggleQuery(ctx context.Context, state *Peco, _ termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doToggleQuery")
		defer g.End()
	}

	q := state.Query()
	if q.Len() == 0 {
		q.RestoreSavedQuery()
	} else {
		q.SaveQuery()
	}

	if state.ExecQuery(nil) {
		return
	}
	state.Hub().SendDrawPrompt(ctx)
}

func doKonamiCommand(ctx context.Context, state *Peco, e termbox.Event) {
	state.Hub().SendStatusMsg(ctx, "All your filters are belongs to us")
}

func doToggleSingleKeyJump(ctx context.Context, state *Peco, e termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doToggleSingleKeyJump")
		defer g.End()
	}
	state.ToggleSingleKeyJumpMode()
}

func doToggleViewArround(ctx context.Context, state *Peco, e termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doToggleViewArround")
		defer g.End()
	}
	q := state.Query()

	if q.Len() > 0 {
		l, err := state.CurrentLineBuffer().LineAt(state.Location().LineNumber())
		if err != nil {
			return
		}
		currentLine := l.ID()

		doDeleteAll(ctx, state, e)
		state.Hub().SendPaging(ctx, ui.JumpToLineRequest(currentLine))
	}
}

func doGoToNextSelection(ctx context.Context, state *Peco, _ termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doGoToNextSelection")
		defer g.End()
	}

	selection := state.Selection()

	if selection.Len() == 0 {
		state.Hub().SendStatusMsg(ctx, "No Selection")
		return
	}

	b := state.CurrentLineBuffer()
	l, err := b.LineAt(state.Location().LineNumber())
	if err != nil {
		return
	}
	currentLine := l.ID()
	nextLine := uint64(math.MaxUint64)
	firstLine := uint64(math.MaxUint64)
	found := false

	selection.Ascend(func(it btree.Item) bool {
		selectedLine := it.(line.Line)
		if selectedLine.ID() > currentLine {
			if selectedLine.ID() < nextLine {
				nextLine = selectedLine.ID()
				found = true
			}
		}

		if selectedLine.ID() <= firstLine {
			firstLine = selectedLine.ID()
		}

		return true
	})

	if found {
		state.Hub().SendStatusMsg(ctx, "Next Selection")
		state.Hub().SendPaging(ctx, ui.ToScrollFirstItem)
		state.Hub().SendPaging(ctx, ui.JumpToLineRequest(nextLine))
	} else {
		state.Hub().SendStatusMsg(ctx, "Next Selection (first)")
		state.Hub().SendPaging(ctx, ui.ToScrollFirstItem)
		state.Hub().SendPaging(ctx, ui.JumpToLineRequest(firstLine))
	}
}

func doGoToPreviousSelection(ctx context.Context, state *Peco, _ termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doGoToPreviousSelection")
		defer g.End()
	}

	selection := state.Selection()

	if selection.Len() == 0 {
		state.Hub().SendStatusMsg(ctx, "No Selection")
		return
	}

	b := state.CurrentLineBuffer()
	l, err := b.LineAt(state.Location().LineNumber())
	if err != nil {
		return
	}
	currentLine := l.ID()
	previousLine := uint64(0)
	lastLine := uint64(math.MaxUint64)
	found := false

	selection.Ascend(func(it btree.Item) bool {
		selectedLine := it.(line.Line)
		if selectedLine.ID() < currentLine {
			if selectedLine.ID() > previousLine {
				previousLine = selectedLine.ID()
				found = true
			}
		}

		if selectedLine.ID() >= lastLine {
			lastLine = selectedLine.ID()
		}

		return true
	})

	if found {
		state.Hub().SendStatusMsg(ctx, "Previous Selection")
		state.Hub().SendPaging(ctx, ui.ToScrollFirstItem)
		state.Hub().SendPaging(ctx, ui.JumpToLineRequest(previousLine))
	} else {
		state.Hub().SendStatusMsg(ctx, "Previous Selection (first)")
		state.Hub().SendPaging(ctx, ui.ToScrollFirstItem)
		state.Hub().SendPaging(ctx, ui.JumpToLineRequest(lastLine))
	}
}

func doSingleKeyJump(ctx context.Context, state *Peco, e termbox.Event) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "doSingleKeyJump %c", e.Ch)
		defer g.End()
	}
	index, ok := state.SingleKeyJumpIndex(e.Ch)
	if !ok {
		// Couldn't find it? Do nothing
		return
	}

	toplevel, _ := ctx.Value(isTopLevelActionCall).(bool)
	state.Hub().Batch(ctx, func(ctx context.Context) {
		ctx = context.WithValue(ctx, isTopLevelActionCall, false)
		state.Hub().SendPaging(ctx, ui.JumpToLineRequest(index))
		doFinish(ctx, state, e)
	}, toplevel)
}

func makeCombinedAction(actions ...Action) ActionFunc {
	return ActionFunc(func(ctx context.Context, state *Peco, e termbox.Event) {
		toplevel, _ := ctx.Value(isTopLevelActionCall).(bool)
		state.Hub().Batch(ctx, func(ctx context.Context) {
			ctx = context.WithValue(ctx, isTopLevelActionCall, false)
			for _, a := range actions {
				a.Execute(ctx, state, e)
			}
		}, toplevel)
	})
}
