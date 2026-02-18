package peco

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"unicode"

	"context"

	"github.com/google/btree"
	"github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
)

// Action describes an action that can be executed upon receiving user input.
type Action interface {
	Execute(context.Context, *Peco, Event)
}

// ActionFunc is a type of Action that is basically just a callback.
type ActionFunc func(context.Context, *Peco, Event)

// ActionMap is the interface for dispatching actions based on key events.
type ActionMap interface {
	ExecuteAction(context.Context, *Peco, Event) error
}

// This is the global map of canonical action name to actions
var nameToActions map[string]Action

// This is the default keybinding used by NewKeymap()
var defaultKeyBinding map[string]Action

// execQueryAndDraw runs ExecQuery and, if the query was non-empty
// (ExecQuery returns false), sends a draw-prompt message.
func execQueryAndDraw(ctx context.Context, state *Peco) {
	if state.ExecQuery(ctx, nil) {
		return
	}
	state.Hub().SendDrawPrompt(ctx)
}

// Execute fulfills the Action interface for AfterFunc
func (a ActionFunc) Execute(ctx context.Context, state *Peco, e Event) {
	a(ctx, state, e)
}

func (a ActionFunc) registerKeySequence(k keyseq.KeyList) {
	defaultKeyBinding[k.String()] = a
}

// Register registers `a` into the global action registry by the name
// `name`, and maps to default keys via `defaultKeys`. Called during
// package init() to set up built-in actions.
func (a ActionFunc) Register(name string, defaultKeys ...keyseq.KeyType) {
	nameToActions["peco."+name] = a
	for _, k := range defaultKeys {
		a.registerKeySequence(keyseq.KeyList{keyseq.NewKeyFromKey(k)})
	}
}

// RegisterKeySequence registers the action to be mapped against a
// multi-key sequence. Called during package init() for actions like
// KonamiCommand.
func (a ActionFunc) RegisterKeySequence(name string, k keyseq.KeyList) {
	nameToActions["peco."+name] = a
	a.registerKeySequence(k)
}

func wrapDeprecated(fn func(context.Context, *Peco, Event), oldName, newName string) ActionFunc {
	return ActionFunc(func(ctx context.Context, state *Peco, e Event) {
		state.Hub().SendStatusMsg(ctx, fmt.Sprintf("%s is deprecated. Use %s", oldName, newName), 0)
		fn(ctx, state, e)
	})
}

// makePagingAction creates an ActionFunc that sends the given PagingRequest.
func makePagingAction(req hub.PagingRequest) ActionFunc {
	return ActionFunc(func(ctx context.Context, state *Peco, _ Event) {
		state.Hub().SendPaging(ctx, req)
	})
}

func init() {
	// Build the global maps
	nameToActions = map[string]Action{}
	defaultKeyBinding = map[string]Action{}

	ActionFunc(doInvertSelection).Register("InvertSelection")
	ActionFunc(doBeginningOfLine).Register("BeginningOfLine", keyseq.KeyCtrlA)
	ActionFunc(doBackwardChar).Register("BackwardChar", keyseq.KeyCtrlB)
	ActionFunc(doBackwardWord).Register("BackwardWord")
	ActionFunc(doCancel).Register("Cancel", keyseq.KeyCtrlC, keyseq.KeyEsc)
	ActionFunc(doDeleteAll).Register("DeleteAll")
	ActionFunc(doDeleteBackwardChar).Register(
		"DeleteBackwardChar",
		keyseq.KeyBackspace,
		keyseq.KeyBackspace2,
	)
	ActionFunc(doDeleteBackwardWord).Register(
		"DeleteBackwardWord",
		keyseq.KeyCtrlW,
	)
	ActionFunc(doDeleteForwardChar).Register("DeleteForwardChar", keyseq.KeyCtrlD)
	ActionFunc(doDeleteForwardWord).Register("DeleteForwardWord")
	ActionFunc(doEndOfFile).Register("EndOfFile")
	ActionFunc(doEndOfLine).Register("EndOfLine", keyseq.KeyCtrlE)
	ActionFunc(doFinish).Register("Finish", keyseq.KeyEnter)
	ActionFunc(doForwardChar).Register("ForwardChar", keyseq.KeyCtrlF)
	ActionFunc(doForwardWord).Register("ForwardWord")
	ActionFunc(doKillEndOfLine).Register("KillEndOfLine", keyseq.KeyCtrlK)
	ActionFunc(doKillBeginningOfLine).Register("KillBeginningOfLine", keyseq.KeyCtrlU)
	ActionFunc(doRotateFilter).Register("RotateFilter", keyseq.KeyCtrlR)
	wrapDeprecated(doRotateFilter, "RotateMatcher", "RotateFilter").Register("RotateMatcher")
	ActionFunc(doBackToInitialFilter).Register("BackToInitialFilter")

	selectUp := makePagingAction(hub.ToLineAbove)
	selectDown := makePagingAction(hub.ToLineBelow)
	scrollPageUp := makePagingAction(hub.ToScrollPageUp)
	scrollPageDown := makePagingAction(hub.ToScrollPageDown)

	selectUp.Register("SelectUp", keyseq.KeyArrowUp, keyseq.KeyCtrlP)
	wrapDeprecated(selectDown, "SelectNext", "SelectUp/SelectDown").Register("SelectNext")

	scrollPageDown.Register("ScrollPageDown", keyseq.KeyArrowRight, keyseq.KeyPgdn)
	wrapDeprecated(scrollPageDown, "SelectNextPage", "ScrollPageDown/ScrollPageUp").Register("SelectNextPage")

	selectDown.Register("SelectDown", keyseq.KeyArrowDown, keyseq.KeyCtrlN)
	wrapDeprecated(selectUp, "SelectPrevious", "SelectUp/SelectDown").Register("SelectPrevious")

	scrollPageUp.Register("ScrollPageUp", keyseq.KeyArrowLeft, keyseq.KeyPgup)
	wrapDeprecated(scrollPageUp, "SelectPreviousPage", "ScrollPageDown/ScrollPageUp").Register("SelectPreviousPage")

	makePagingAction(hub.ToScrollLeft).Register("ScrollLeft")
	makePagingAction(hub.ToScrollRight).Register("ScrollRight")

	makePagingAction(hub.ToScrollFirstItem).Register("ScrollFirstItem", keyseq.KeyHome)
	makePagingAction(hub.ToScrollLastItem).Register("ScrollLastItem", keyseq.KeyEnd)

	ActionFunc(doToggleSelection).Register("ToggleSelection")
	ActionFunc(doToggleSelectionAndSelectNext).Register(
		"ToggleSelectionAndSelectNext",
		keyseq.KeyCtrlSpace,
	)
	ActionFunc(doSelectNone).Register(
		"SelectNone",
		keyseq.KeyCtrlG,
	)
	ActionFunc(doSelectAll).Register("SelectAll")
	ActionFunc(doSelectVisible).Register("SelectVisible")
	wrapDeprecated(doToggleRangeMode, "ToggleSelectMode", "ToggleRangeMode").Register("ToggleSelectMode")
	wrapDeprecated(doCancelRangeMode, "CancelSelectMode", "CancelRangeMode").Register("CancelSelectMode")
	ActionFunc(doToggleRangeMode).Register("ToggleRangeMode")
	ActionFunc(doCancelRangeMode).Register("CancelRangeMode")
	ActionFunc(doToggleQuery).Register("ToggleQuery", keyseq.KeyCtrlT)
	ActionFunc(doRefreshScreen).Register("RefreshScreen", keyseq.KeyCtrlL)
	ActionFunc(doToggleSingleKeyJump).Register("ToggleSingleKeyJump")

	ActionFunc(doToggleViewAround).Register("ViewAround", keyseq.KeyCtrlV)
	wrapDeprecated(doToggleViewAround, "ViewArround", "ViewAround").Register("ViewArround")

	ActionFunc(doFreezeResults).Register("FreezeResults")
	ActionFunc(doUnfreezeResults).Register("UnfreezeResults")

	ActionFunc(doZoomIn).Register("ZoomIn")
	ActionFunc(doZoomOut).Register("ZoomOut")

	ActionFunc(doGoToNextSelection).Register("GoToNextSelection")
	ActionFunc(doGoToPreviousSelection).Register("GoToPreviousSelection", keyseq.KeyCtrlJ)

	ActionFunc(doKonamiCommand).RegisterKeySequence(
		"KonamiCommand",
		keyseq.KeyList{
			keyseq.Key{Modifier: 0, Key: keyseq.KeyCtrlX, Ch: 0},
			keyseq.Key{Modifier: 0, Key: keyseq.KeyArrowUp, Ch: 0},
			keyseq.Key{Modifier: 0, Key: keyseq.KeyArrowUp, Ch: 0},
			keyseq.Key{Modifier: 0, Key: keyseq.KeyArrowDown, Ch: 0},
			keyseq.Key{Modifier: 0, Key: keyseq.KeyArrowDown, Ch: 0},
			keyseq.Key{Modifier: 0, Key: keyseq.KeyArrowLeft, Ch: 0},
			keyseq.Key{Modifier: 0, Key: keyseq.KeyArrowRight, Ch: 0},
			keyseq.Key{Modifier: 0, Key: keyseq.KeyArrowLeft, Ch: 0},
			keyseq.Key{Modifier: 0, Key: keyseq.KeyArrowRight, Ch: 0},
			keyseq.Key{Modifier: 0, Key: 0, Ch: 'b'},
			keyseq.Key{Modifier: 0, Key: 0, Ch: 'a'},
		},
	)
}

// selectLine marks the line as dirty and adds it to the selection.
func selectLine(l line.Line, s *Selection) {
	l.SetDirty(true)
	s.Add(l)
}

// This is a noop action
func doNothing(_ context.Context, _ *Peco, _ Event) {}

// This is an exception to the rule. This does not get registered
// anywhere. You just call it directly
func doAcceptChar(ctx context.Context, state *Peco, e Event) {
	if e.Key == keyseq.KeySpace {
		e.Ch = ' '
	}

	ch := e.Ch
	if ch <= 0 {
		return
	}

	if state.SingleKeyJump().Mode() {
		doSingleKeyJump(ctx, state, e)
		return
	}

	q := state.Query()
	c := state.Caret()

	q.InsertAt(ch, c.Pos())
	c.Move(1)

	h := state.Hub()
	h.SendDrawPrompt(ctx) // Update prompt before running query

	state.ExecQuery(ctx, nil)
}

func doRotateFilter(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doRotateFilter")
		defer g.End()
	}

	filters := state.Filters()
	filters.Rotate()

	execQueryAndDraw(ctx, state)
}

func doBackToInitialFilter(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doBackToInitialFilter")
		defer g.End()
	}

	filters := state.Filters()
	filters.Reset()

	execQueryAndDraw(ctx, state)
}

func doToggleSelection(_ context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doToggleSelection")
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

func doToggleRangeMode(_ context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doToggleRangeMode")
		defer g.End()
	}

	r := state.SelectionRangeStart()
	if r.Valid() {
		r.Reset()
	} else {
		cl := state.Location().LineNumber()
		r.SetValue(cl)
		if l, err := state.CurrentLineBuffer().LineAt(cl); err == nil {
			state.Selection().Add(l)
		}
	}
}

func doCancelRangeMode(_ context.Context, state *Peco, _ Event) {
	state.SelectionRangeStart().Reset()
}

func doSelectNone(ctx context.Context, state *Peco, _ Event) {
	state.Selection().Reset()
	state.Hub().SendDraw(ctx, &hub.DrawOptions{DisableCache: true})
}

func doSelectAll(ctx context.Context, state *Peco, _ Event) {
	selection := state.Selection()
	b := state.CurrentLineBuffer()
	for x := range b.Size() {
		l, err := b.LineAt(x)
		if err != nil {
			continue
		}
		selectLine(l, selection)
	}
	state.Hub().SendDraw(ctx, nil)
}

func doSelectVisible(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doSelectVisible")
		defer g.End()
	}

	b := state.CurrentLineBuffer()
	selection := state.Selection()
	loc := state.Location()
	pc := loc.PageCrop()
	lb := pc.Crop(b)
	for x := range lb.Size() {
		l, err := lb.LineAt(x)
		if err != nil {
			continue
		}
		selectLine(l, selection)
	}
	state.Hub().SendDraw(ctx, nil)
}

type collectResultsError struct{}

func (err collectResultsError) Error() string {
	return "collect results"
}
func (err collectResultsError) CollectResults() bool {
	return true
}
func doFinish(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doFinish")
		defer g.End()
	}

	ccarg := state.execOnFinish
	if len(ccarg) == 0 {
		state.Exit(collectResultsError{})
		return
	}

	sel := NewSelection()
	state.Selection().Copy(sel)
	if sel.Len() == 0 {
		if l, err := state.CurrentLineBuffer().LineAt(state.Location().LineNumber()); err == nil {
			sel.Add(l)
		}
	}

	var stdin bytes.Buffer
	sel.Ascend(func(it btree.Item) bool {
		line, ok := it.(line.Line)
		if !ok {
			return true
		}
		stdin.WriteString(line.Buffer())
		stdin.WriteRune('\n')
		return true
	})

	var err error
	state.Hub().SendStatusMsg(ctx, "Executing "+ccarg, 0)
	cmd := util.Shell(ctx, ccarg)
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
	if err := state.screen.Resume(ctx); err != nil {
		state.Exit(fmt.Errorf("failed to resume screen: %w", err))
		return
	}
	state.Hub().SendDraw(ctx, &hub.DrawOptions{DisableCache: true})
	if err != nil {
		// bail out, or otherwise the user cannot know what happened
		state.Exit(fmt.Errorf("failed to execute command: %w", err))
	}
}

func doCancel(ctx context.Context, state *Peco, e Event) {
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
	if state.onCancel == OnCancelError {
		err = setExitStatus(err, 1)
	}
	state.Exit(err)
}

// batchAction runs fn inside a Hub.Batch. Re-entrant locking is
// handled automatically by Hub.Batch via context detection.
func batchAction(ctx context.Context, state *Peco, fn func(context.Context)) {
	state.Hub().Batch(ctx, fn)
}

func doToggleSelectionAndSelectNext(ctx context.Context, state *Peco, e Event) {
	batchAction(ctx, state, func(ctx context.Context) {
		doToggleSelection(ctx, state, e)
		if state.LayoutType() != LayoutTypeBottomUp {
			state.Hub().SendPaging(ctx, hub.ToLineBelow)
		} else {
			state.Hub().SendPaging(ctx, hub.ToLineAbove)
		}
	})
}

func doInvertSelection(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doInvertSelection")
		defer g.End()
	}

	selection := state.Selection()
	b := state.CurrentLineBuffer()

	for x := range b.Size() {
		l, err := b.LineAt(x)
		if err != nil {
			continue
		}
		if selection.Has(l) {
			l.SetDirty(true)
			selection.Remove(l)
		} else {
			selectLine(l, selection)
		}
	}

	state.Hub().SendDraw(ctx, nil)
}

func doDeleteBackwardWord(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doDeleteBackwardWord")
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
	execQueryAndDraw(ctx, state)
}

func doForwardWord(ctx context.Context, state *Peco, _ Event) {
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

func doBackwardWord(ctx context.Context, state *Peco, _ Event) {
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
	for {
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
			continue
		}
		break
	}

	// Now look for a space
	for pos := c.Pos(); pos > 0; pos-- {
		if unicode.IsSpace(q.RuneAt(pos)) {
			c.SetPos(pos + 1)
			return
		}
	}

	// not found. just move to the beginning of the buffer
	c.SetPos(0)
}

func doForwardChar(ctx context.Context, state *Peco, _ Event) {
	c := state.Caret()
	if c.Pos() >= state.Query().Len() {
		return
	}
	c.Move(1)
	state.Hub().SendDrawPrompt(ctx)
}

func doBackwardChar(ctx context.Context, state *Peco, _ Event) {
	c := state.Caret()
	if c.Pos() <= 0 {
		return
	}
	c.Move(-1)
	state.Hub().SendDrawPrompt(ctx)
}

func doDeleteForwardWord(ctx context.Context, state *Peco, _ Event) {
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

	execQueryAndDraw(ctx, state)
}

func doBeginningOfLine(ctx context.Context, state *Peco, _ Event) {
	state.Caret().SetPos(0)
	state.Hub().SendDrawPrompt(ctx)
}

func doEndOfLine(ctx context.Context, state *Peco, _ Event) {
	state.Caret().SetPos(state.Query().Len())
	state.Hub().SendDrawPrompt(ctx)
}

func doEndOfFile(ctx context.Context, state *Peco, e Event) {
	if state.Query().Len() > 0 {
		doDeleteForwardChar(ctx, state, e)
	} else {
		doCancel(ctx, state, e)
	}
}

func doKillBeginningOfLine(ctx context.Context, state *Peco, _ Event) {
	q := state.Query()
	q.DeleteRange(0, state.Caret().Pos())
	state.Caret().SetPos(0)
	execQueryAndDraw(ctx, state)
}

func doKillEndOfLine(ctx context.Context, state *Peco, _ Event) {
	if state.Query().Len() <= state.Caret().Pos() {
		return
	}

	q := state.Query()
	q.DeleteRange(state.Caret().Pos(), q.Len())
	execQueryAndDraw(ctx, state)
}

func doDeleteAll(ctx context.Context, state *Peco, _ Event) {
	state.Query().Reset()
	state.ExecQuery(ctx, nil)
}

func doDeleteForwardChar(ctx context.Context, state *Peco, _ Event) {
	q := state.Query()
	c := state.Caret()
	if q.Len() <= c.Pos() {
		return
	}

	pos := c.Pos()
	q.DeleteRange(pos, pos+1)

	execQueryAndDraw(ctx, state)
}

func doDeleteBackwardChar(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doDeleteBackwardChar")
		defer g.End()
	}

	q := state.Query()
	c := state.Caret()
	qlen := q.Len()
	if qlen <= 0 {
		if pdebug.Enabled {
			pdebug.Printf("doDeleteBackwardChar: QueryLen <= 0, do nothing")
		}
		return
	}

	pos := c.Pos()
	if pos == 0 {
		if pdebug.Enabled {
			pdebug.Printf("doDeleteBackwardChar: Already at position 0")
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

	execQueryAndDraw(ctx, state)
}

func doRefreshScreen(ctx context.Context, state *Peco, _ Event) {
	state.Hub().SendDraw(ctx, &hub.DrawOptions{DisableCache: true, ForceSync: true})
}

func doToggleQuery(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doToggleQuery")
		defer g.End()
	}

	q := state.Query()
	if q.Len() == 0 {
		q.RestoreSavedQuery()
	} else {
		q.SaveQuery()
	}

	execQueryAndDraw(ctx, state)
}

func doKonamiCommand(ctx context.Context, state *Peco, _ Event) {
	state.Hub().SendStatusMsg(ctx, "All your filters are belongs to us", 0)
}

func doToggleSingleKeyJump(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doToggleSingleKeyJump")
		defer g.End()
	}
	state.ToggleSingleKeyJumpMode(ctx)
}

func doToggleViewAround(ctx context.Context, state *Peco, e Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doToggleViewAround")
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
		state.Hub().SendPaging(ctx, hub.JumpToLineRequest(currentLine))
	}
}

func doGoToNextSelection(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doGoToNextSelection")
		defer g.End()
	}
	doGoToAdjacentSelection(ctx, state, true)
}

func doGoToPreviousSelection(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doGoToPreviousSelection")
		defer g.End()
	}
	doGoToAdjacentSelection(ctx, state, false)
}

// doGoToAdjacentSelection navigates to the next (forward=true) or previous
// (forward=false) selected line, wrapping around if necessary.
func doGoToAdjacentSelection(ctx context.Context, state *Peco, forward bool) {
	selection := state.Selection()

	if selection.Len() == 0 {
		state.Hub().SendStatusMsg(ctx, "No Selection", 0)
		return
	}

	b := state.CurrentLineBuffer()
	l, err := b.LineAt(state.Location().LineNumber())
	if err != nil {
		return
	}
	currentLine := l.ID()

	// target: the nearest selected line in the desired direction.
	// wrapTarget: the line to jump to when wrapping around
	//   (smallest ID for forward, largest ID for backward).
	var target, wrapTarget uint64
	if forward {
		target = math.MaxUint64
		wrapTarget = math.MaxUint64
	}
	found := false

	selection.Ascend(func(it btree.Item) bool {
		l, ok := it.(line.Line)
		if !ok {
			return true
		}
		id := l.ID()
		if forward {
			if id > currentLine && id < target {
				target = id
				found = true
			}
			if id <= wrapTarget {
				wrapTarget = id
			}
		} else {
			if id < currentLine && id > target {
				target = id
				found = true
			}
			if id > wrapTarget {
				wrapTarget = id
			}
		}
		return true
	})

	label := "Next"
	if !forward {
		label = "Previous"
	}

	if found {
		state.Hub().SendStatusMsg(ctx, label+" Selection", 0)
		state.Hub().SendPaging(ctx, hub.ToScrollFirstItem)
		state.Hub().SendPaging(ctx, hub.JumpToLineRequest(target))
	} else {
		state.Hub().SendStatusMsg(ctx, label+" Selection (first)", 0)
		state.Hub().SendPaging(ctx, hub.ToScrollFirstItem)
		state.Hub().SendPaging(ctx, hub.JumpToLineRequest(wrapTarget))
	}
}

// resetQueryState clears the query, resets the caret, and (unless sticky
// selection is enabled) clears the selection. Used by freeze/unfreeze to
// return to a clean query state.
func resetQueryState(state *Peco) {
	state.Query().Reset()
	state.Caret().SetPos(0)
	if !state.config.StickySelection {
		state.Selection().Reset()
	}
}

func doFreezeResults(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doFreezeResults")
		defer g.End()
	}

	b := state.CurrentLineBuffer()
	if b.Size() == 0 {
		state.Hub().SendStatusMsg(ctx, "Nothing to freeze", 0)
		return
	}

	frozen := NewMemoryBuffer(b.Size())
	for i := range b.Size() {
		if l, err := b.LineAt(i); err == nil {
			frozen.AppendLine(l)
		}
	}
	frozen.MarkComplete()

	state.Frozen().Set(frozen)
	resetQueryState(state)
	state.SetCurrentLineBuffer(ctx, frozen)
	state.Hub().SendStatusMsg(ctx, "Results frozen", 0)
	state.Hub().SendDrawPrompt(ctx)
}

func doUnfreezeResults(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doUnfreezeResults")
		defer g.End()
	}

	if state.Frozen().Source() == nil {
		state.Hub().SendStatusMsg(ctx, "No frozen results", 0)
		return
	}

	state.Frozen().Clear()
	resetQueryState(state)
	state.ResetCurrentLineBuffer(ctx)
	state.Hub().SendStatusMsg(ctx, "Results unfrozen", 0)
	state.Hub().SendDrawPrompt(ctx)
}

func doZoomIn(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doZoomIn")
		defer g.End()
	}

	// Already zoomed in?
	if state.Zoom().Buffer() != nil {
		state.Hub().SendStatusMsg(ctx, "Already zoomed in", 0)
		return
	}

	// Get the current line buffer
	currentBuf := state.CurrentLineBuffer()

	// If the current buffer is the source (no active filter), nothing to zoom into
	if currentBuf == state.source {
		state.Hub().SendStatusMsg(ctx, "Nothing to zoom into", 0)
		return
	}

	source := state.source
	contextSize := 3

	contextBuf := NewContextBuffer(currentBuf, source, contextSize)
	if contextBuf.Size() == 0 {
		state.Hub().SendStatusMsg(ctx, "Nothing to zoom into", 0)
		return
	}

	// Save current state for ZoomOut
	loc := state.Location()
	curLineNo := loc.LineNumber()
	state.Zoom().Set(currentBuf, curLineNo)

	// Map cursor to the new context buffer position
	newLineNo := 0
	indices := contextBuf.MatchEntryIndices()
	if curLineNo >= 0 && curLineNo < len(indices) && indices[curLineNo] >= 0 {
		newLineNo = indices[curLineNo]
	}

	state.setCurrentLineBufferNoNotify(contextBuf)

	loc.SetLineNumber(newLineNo)
	state.Hub().SendDraw(ctx, &hub.DrawOptions{DisableCache: true})
}

func doZoomOut(ctx context.Context, state *Peco, _ Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doZoomOut")
		defer g.End()
	}

	preZoom := state.Zoom().Buffer()
	if preZoom == nil {
		state.Hub().SendStatusMsg(ctx, "Not zoomed in", 0)
		return
	}

	loc := state.Location()
	savedLineNo := state.Zoom().LineNo()

	state.setCurrentLineBufferNoNotify(preZoom)

	loc.SetLineNumber(savedLineNo)
	state.Zoom().Clear()
	state.Hub().SendDraw(ctx, &hub.DrawOptions{DisableCache: true})
}

func doSingleKeyJump(ctx context.Context, state *Peco, e Event) {
	if pdebug.Enabled {
		g := pdebug.Marker("doSingleKeyJump %c", e.Ch)
		defer g.End()
	}
	index, ok := state.SingleKeyJump().Index(e.Ch)
	if !ok {
		// Couldn't find it? Do nothing
		return
	}

	batchAction(ctx, state, func(ctx context.Context) {
		state.Hub().SendPaging(ctx, hub.JumpToLineRequest(index))
		doFinish(ctx, state, e)
	})
}

func makeCombinedAction(actions ...Action) ActionFunc {
	return ActionFunc(func(ctx context.Context, state *Peco, e Event) {
		batchAction(ctx, state, func(ctx context.Context) {
			for _, a := range actions {
				a.Execute(ctx, state, e)
			}
		})
	})
}
