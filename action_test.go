package peco

import (
	"fmt"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"context"

	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/line"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingHub wraps nullHub but records SendPaging, SendStatusMsg, and SendDraw calls.
type recordingHub struct {
	nullHub
	mu         sync.Mutex
	pagingArgs []hub.PagingRequest
	statusMsgs []string
	drawArgs   []*hub.DrawOptions
}

func (h *recordingHub) SendPaging(_ context.Context, v hub.PagingRequest) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pagingArgs = append(h.pagingArgs, v)
}

func (h *recordingHub) SendStatusMsg(_ context.Context, msg string, _ time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statusMsgs = append(h.statusMsgs, msg)
}

func (h *recordingHub) SendDraw(_ context.Context, v *hub.DrawOptions) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.drawArgs = append(h.drawArgs, v)
}

func (h *recordingHub) getDrawArgs() []*hub.DrawOptions {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]*hub.DrawOptions, len(h.drawArgs))
	copy(dst, h.drawArgs)
	return dst
}

func (h *recordingHub) getPagingArgs() []hub.PagingRequest {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]hub.PagingRequest, len(h.pagingArgs))
	copy(dst, h.pagingArgs)
	return dst
}

func (h *recordingHub) getStatusMsgs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]string, len(h.statusMsgs))
	copy(dst, h.statusMsgs)
	return dst
}

func (h *recordingHub) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pagingArgs = nil
	h.statusMsgs = nil
	h.drawArgs = nil
}

// failingBuffer is a Buffer where LineAt returns an error for specific indices.
// This is used to test that doSelectAll and doInvertSelection don't panic when
// LineAt fails (returning nil line).
type failingBuffer struct {
	lines  []line.Line
	failAt map[int]bool // indices where LineAt should return an error
}

func (b *failingBuffer) linesInRange(start, end int) []line.Line {
	return b.lines[start:end]
}

func (b *failingBuffer) LineAt(i int) (line.Line, error) {
	if b.failAt[i] {
		return nil, fmt.Errorf("simulated failure at index %d", i)
	}
	if i < 0 || i >= len(b.lines) {
		return nil, fmt.Errorf("index out of range: %d", i)
	}
	return b.lines[i], nil
}

func (b *failingBuffer) Size() int {
	return len(b.lines)
}

func TestDoSelectAllWithLineAtError(t *testing.T) {
	state := New()
	state.screen = NewDummyScreen()
	state.readConfigFn = func(*Config, string) error { return nil }

	rh := &recordingHub{}
	state.hub = rh

	// Create lines and a buffer where index 1 fails
	l0 := line.NewRaw(1, "line0", false, false)
	l1 := line.NewRaw(2, "line1", false, false)
	l2 := line.NewRaw(3, "line2", false, false)

	buf := &failingBuffer{
		lines:  []line.Line{l0, l1, l2},
		failAt: map[int]bool{1: true},
	}
	state.currentLineBuffer = buf

	// This should NOT panic despite LineAt returning nil for index 1.
	require.NotPanics(t, func() {
		doSelectAll(context.Background(), state, Event{})
	}, "doSelectAll must not panic when LineAt returns an error")

	// Lines 0 and 2 should be selected; line 1 (failed) should be skipped.
	sel := state.Selection()
	require.True(t, sel.Has(l0), "line 0 should be selected")
	require.False(t, sel.Has(l1), "line 1 should not be selected (LineAt failed)")
	require.True(t, sel.Has(l2), "line 2 should be selected")
}

func TestDoInvertSelectionWithLineAtError(t *testing.T) {
	state := New()
	state.screen = NewDummyScreen()
	state.readConfigFn = func(*Config, string) error { return nil }

	rh := &recordingHub{}
	state.hub = rh

	// Create lines and a buffer where index 1 fails
	l0 := line.NewRaw(1, "line0", false, false)
	l1 := line.NewRaw(2, "line1", false, false)
	l2 := line.NewRaw(3, "line2", false, false)

	buf := &failingBuffer{
		lines:  []line.Line{l0, l1, l2},
		failAt: map[int]bool{1: true},
	}
	state.currentLineBuffer = buf

	// Pre-select lines 0 and 2, so inversion should deselect them
	state.Selection().Add(l0)
	state.Selection().Add(l2)

	// This should NOT panic despite LineAt returning nil for index 1.
	require.NotPanics(t, func() {
		doInvertSelection(context.Background(), state, Event{})
	}, "doInvertSelection must not panic when LineAt returns an error")

	// Line 0 was selected → should now be deselected.
	// Line 1 failed → skip (remain unselected).
	// Line 2 was selected → should now be deselected.
	sel := state.Selection()
	require.False(t, sel.Has(l0), "line 0 was selected, should be deselected after inversion")
	require.False(t, sel.Has(l1), "line 1 should not be affected (LineAt failed)")
	require.False(t, sel.Has(l2), "line 2 was selected, should be deselected after inversion")
}

func TestActionFunc(t *testing.T) {
	called := 0
	af := ActionFunc(func(_ context.Context, _ *Peco, _ Event) {
		called++
	})
	af.Execute(context.TODO(), nil, Event{})
	if !assert.Equal(t, called, 1, "Expected ActionFunc to be called once, but it got called %d times", called) {
		return
	}
}

func TestPagingActions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		action   string
		expected hub.PagingRequest
	}{
		{"SelectUp", "peco.SelectUp", hub.ToLineAbove},
		{"SelectDown", "peco.SelectDown", hub.ToLineBelow},
		{"ScrollPageUp", "peco.ScrollPageUp", hub.ToScrollPageUp},
		{"ScrollPageDown", "peco.ScrollPageDown", hub.ToScrollPageDown},
		{"ScrollLeft", "peco.ScrollLeft", hub.ToScrollLeft},
		{"ScrollRight", "peco.ScrollRight", hub.ToScrollRight},
		{"ScrollFirstItem", "peco.ScrollFirstItem", hub.ToScrollFirstItem},
		{"ScrollLastItem", "peco.ScrollLastItem", hub.ToScrollLastItem},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rHub := &recordingHub{}
			state := New()
			state.hub = rHub
			state.selection = NewSelection()
			state.currentLineBuffer = NewMemoryBuffer(0)

			action, ok := nameToActions[tt.action]
			require.True(t, ok, "action %s should exist", tt.action)

			action.Execute(ctx, state, Event{})

			pagingArgs := rHub.getPagingArgs()
			require.Len(t, pagingArgs, 1, "expected exactly one SendPaging call")
			require.Equal(t, tt.expected, pagingArgs[0],
				"expected paging request %v, got %v", tt.expected, pagingArgs[0])
		})
	}
}

func TestActionNames(t *testing.T) {
	// These names MUST exist
	names := []string{
		"peco.ForwardChar",
		"peco.BackwardChar",
		"peco.ForwardWord",
		"peco.BackwardWord",
		"peco.BeginningOfLine",
		"peco.EndOfLine",
		"peco.EndOfFile",
		"peco.DeleteForwardChar",
		"peco.DeleteBackwardChar",
		"peco.DeleteForwardWord",
		"peco.DeleteBackwardWord",
		"peco.KillEndOfLine",
		"peco.DeleteAll",
		"peco.SelectPreviousPage",
		"peco.SelectNextPage",
		"peco.SelectPrevious",
		"peco.SelectNext",
		"peco.ToggleSelection",
		"peco.ToggleSelectionAndSelectNext",
		"peco.RotateMatcher",
		"peco.Finish",
		"peco.Cancel",
		"peco.FreezeResults",
		"peco.UnfreezeResults",
		"peco.ZoomIn",
		"peco.ZoomOut",
	}
	for _, name := range names {
		if _, ok := nameToActions[name]; !ok {
			t.Errorf("Action %s should exist, but it does not", name)
		}
	}
}

func TestViewAroundActionName(t *testing.T) {
	// The correct spelling "ViewAround" must be registered.
	_, ok := nameToActions["peco.ViewAround"]
	require.True(t, ok, "peco.ViewAround must be registered as an action name")

	// The old misspelled name "ViewArround" must also work for backward compatibility.
	_, ok = nameToActions["peco.ViewArround"]
	require.True(t, ok, "peco.ViewArround must remain registered for backward compatibility")
}

func expectCaretPos(t *testing.T, c *Caret, expect int) bool {
	return assert.Equal(t, expect, c.Pos(), "Expected caret position %d, got %d", expect, c.Pos())
}

func expectQueryString(t *testing.T, q *Query, expect string) bool {
	return assert.Equal(t, expect, q.String(), "Expected '%s', got '%s'", expect, q.String())
}

func TestDoDeleteForwardChar(t *testing.T) {
	state, ctx := setupPecoTest(t)
	q := state.Query()
	c := state.Caret()

	q.Set("Hello, World!")
	c.SetPos(5)

	doDeleteForwardChar(ctx, state, Event{})

	if !expectQueryString(t, q, "Hello World!") {
		return
	}
	if !expectCaretPos(t, c, 5) {
		return
	}

	c.SetPos(q.Len())
	doDeleteForwardChar(ctx, state, Event{})

	expectQueryString(t, q, "Hello World!")
	expectCaretPos(t, c, q.Len())

	c.SetPos(0)
	doDeleteForwardChar(ctx, state, Event{})

	expectQueryString(t, q, "ello World!")
	expectCaretPos(t, c, 0)
}

func TestDoDeleteForwardWord(t *testing.T) {
	state, ctx := setupPecoTest(t)
	q := state.Query()
	c := state.Caret()

	q.Set("Hello, World!")
	c.SetPos(5)

	// delete the comma
	doDeleteForwardWord(ctx, state, Event{})
	if !expectQueryString(t, q, "Hello World!") {
		return
	}

	if !expectCaretPos(t, c, 5) {
		return
	}

	// at the end of the query, should not delete anything
	c.SetPos(q.Len())
	doDeleteForwardWord(ctx, state, Event{})

	if !expectQueryString(t, q, "Hello World!") {
		return
	}
	if !expectCaretPos(t, c, q.Len()) {
		return
	}

	// back to the first column, should delete 'Hello'
	c.SetPos(0)
	doDeleteForwardWord(ctx, state, Event{})

	if !expectQueryString(t, q, " World!") {
		return
	}

	if !expectCaretPos(t, c, 0) {
		return
	}

	// should delete "World"
	c.SetPos(1)
	doDeleteForwardWord(ctx, state, Event{})

	if !expectQueryString(t, q, " ") {
		return
	}
}

func TestDoDeleteBackwardChar(t *testing.T) {
	state, ctx := setupPecoTest(t)
	q := state.Query()
	c := state.Caret()

	q.Set("Hello, World!")
	c.SetPos(5)

	doDeleteBackwardChar(ctx, state, Event{})

	expectQueryString(t, q, "Hell, World!")
	expectCaretPos(t, c, 4)

	c.SetPos(q.Len())
	doDeleteBackwardChar(ctx, state, Event{})

	expectQueryString(t, q, "Hell, World")
	expectCaretPos(t, c, q.Len())

	c.SetPos(0)
	doDeleteBackwardChar(ctx, state, Event{})

	expectQueryString(t, q, "Hell, World")
	expectCaretPos(t, c, 0)
}

func TestDoDeleteBackwardWord(t *testing.T) {
	state, ctx := setupPecoTest(t)
	q := state.Query()
	c := state.Caret()

	// In case of an overflow (bug)
	q.Set("foo")
	c.SetPos(5)
	doDeleteBackwardWord(ctx, state, Event{})

	// https://github.com/peco/peco/pull/184#issuecomment-54026739

	// Case 1. " foo<caret>" -> " "
	q.Set(" foo")
	c.SetPos(4)
	doDeleteBackwardWord(ctx, state, Event{})

	if !expectQueryString(t, q, " ") {
		return
	}

	if !expectCaretPos(t, c, 1) {
		return
	}

	// Case 2. "foo bar<caret>" -> "foo "
	q.Set("foo bar")
	c.SetPos(7)
	doDeleteBackwardWord(ctx, state, Event{})

	if !expectQueryString(t, q, "foo ") {
		return
	}

	if !expectCaretPos(t, c, 4) {
		return
	}
}

func writeQueryToPrompt(t *testing.T, screen Screen, message string) {
	for str := message; true; {
		r, size := utf8.DecodeRuneInString(str)
		if r == utf8.RuneError {
			assert.Equal(t, 0, size, "when in error, we should have size == 0")
			return
		}

		if r == ' ' {
			screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeySpace})
		} else {
			screen.SendEvent(Event{Type: EventKey, Ch: r})
		}
		str = str[size:]
	}
}

func TestDoAcceptChar(t *testing.T) {
	state, _ := setupPecoTest(t)

	message := "Hello, World!"
	writeQueryToPrompt(t, state.screen, message)
	require.Eventually(t, func() bool {
		return state.Query().String() == message
	}, 5*time.Second, 10*time.Millisecond, "Expected query to be populated as '%s'", message)

	state.Caret().Move(-1 * len("World!"))
	writeQueryToPrompt(t, state.screen, "Cruel ")

	expected := "Hello, Cruel World!"
	require.Eventually(t, func() bool {
		return state.Query().String() == expected
	}, 5*time.Second, 10*time.Millisecond, "Expected query to be populated as '%s'", expected)
}

func TestRotateFilter(t *testing.T) {
	state, _ := setupPecoTest(t)

	size := state.filters.Size()
	if size <= 1 {
		t.Skip("Can't proceed testing, only have 1 filter registered")
		return
	}

	var prev filter.Filter
	first := state.Filters().Current()
	prev = first
	for i := range size {
		prevFilter := prev
		state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlR})

		require.Eventually(t, func() bool {
			return state.Filters().Current() != prevFilter
		}, 5*time.Second, 10*time.Millisecond, "filter should have rotated at iteration %d", i)
		prev = state.Filters().Current()
	}

	if first != prev {
		t.Errorf("should have rotated back to first one, but didn't")
	}

	// Verify ExecQuery is triggered after rotation: type a query, rotate,
	// and verify the filtered buffer updates (proving re-execution).
	sourceSize := state.Source().(Buffer).Size()

	writeQueryToPrompt(t, state.screen, "func")
	require.Eventually(t, func() bool {
		return state.Query().String() == "func"
	}, 5*time.Second, 10*time.Millisecond, "query should be set to 'func'")

	// Wait for filter to produce results (buffer should be smaller than source)
	require.Eventually(t, func() bool {
		buf := state.CurrentLineBuffer()
		return buf.Size() > 0 && buf.Size() < sourceSize
	}, 5*time.Second, 10*time.Millisecond,
		"buffer should be filtered to less than source size (%d)", sourceSize)

	filteredSize := state.CurrentLineBuffer().Size()
	prevFilter := state.Filters().Current()

	// Rotate filter — this calls execQueryAndDraw → ExecQuery
	state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlR})

	require.Eventually(t, func() bool {
		return state.Filters().Current() != prevFilter
	}, 5*time.Second, 10*time.Millisecond, "filter should have rotated")

	// After rotation with non-empty query, ExecQuery re-filters. Buffer
	// should still be filtered (not reset to full source).
	require.Eventually(t, func() bool {
		buf := state.CurrentLineBuffer()
		return buf.Size() > 0 && buf.Size() <= sourceSize
	}, 5*time.Second, 10*time.Millisecond,
		"after rotation, buffer should still be filtered (was %d)", filteredSize)
}

func TestBeginningOfLineAndEndOfLine(t *testing.T) {
	state, _ := setupPecoTest(t)

	message := "Hello, World!"
	writeQueryToPrompt(t, state.screen, message)
	state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlA})

	require.Eventually(t, func() bool {
		return state.Caret().Pos() == 0
	}, 5*time.Second, 10*time.Millisecond, "Expected caret position to be 0")

	state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlE})

	require.Eventually(t, func() bool {
		return state.Caret().Pos() == len(message)
	}, 5*time.Second, 10*time.Millisecond, "Expected caret position to be %d", len(message))
}

func TestBackToInitialFilter(t *testing.T) {
	state, _ := setupPecoTest(t)

	state.config.Keymap["C-q"] = "peco.BackToInitialFilter"
	if !assert.NoError(t, state.populateKeymap(), "populateKeymap expected to succeed") {
		return
	}

	if !assert.Equal(t, state.Filters().Index(), 0, "Expected filter to be at position 0, got %d", state.Filters().Index()) {
		return
	}

	state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlR})
	require.Eventually(t, func() bool {
		return state.Filters().Index() == 1
	}, 5*time.Second, 10*time.Millisecond, "Expected filter to be at position 1")

	state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlQ})
	require.Eventually(t, func() bool {
		return state.Filters().Index() == 0
	}, 5*time.Second, 10*time.Millisecond, "Expected filter to be at position 0")
}

func TestGHIssue574_PreviousSelectionLastLineNotUpdated(t *testing.T) {
	// Issue #574: In doGoToPreviousSelection, lastLine is initialized to
	// math.MaxUint64 and the condition `selectedLine.ID() >= lastLine` can
	// never be true, so lastLine never gets updated. When wrapping around
	// (cursor is before/at the first selected line), it should jump to the
	// last selected line, but instead it jumps to math.MaxUint64.

	ctx := context.Background()

	// Create lines with known IDs.
	// We use IDs 10, 20, 30, 40, 50 for five lines.
	lines := []line.Line{
		line.NewRaw(10, "line-10", false, false),
		line.NewRaw(20, "line-20", false, false),
		line.NewRaw(30, "line-30", false, false),
		line.NewRaw(40, "line-40", false, false),
		line.NewRaw(50, "line-50", false, false),
	}

	// Build a MemoryBuffer containing those lines.
	mb := NewMemoryBuffer(0)
	mb.lines = lines

	rHub := &recordingHub{}

	state := New()
	state.hub = rHub
	state.selection = NewSelection()

	// Set the current line buffer to our prepared buffer.
	state.currentLineBuffer = mb

	// Select lines with IDs 20 and 40.
	state.Selection().Add(lines[1]) // ID=20
	state.Selection().Add(lines[3]) // ID=40

	t.Run("wrap around to last selected line", func(t *testing.T) {
		// Position cursor at line index 0 (ID=10), which is before all
		// selected lines. There is no "previous" selection, so the function
		// should wrap around to the last selected line (ID=40).
		state.Location().SetLineNumber(0)
		rHub.reset()

		doGoToPreviousSelection(ctx, state, Event{})

		statusMsgs := rHub.getStatusMsgs()
		require.NotEmpty(t, statusMsgs, "should have sent a status message")
		require.Equal(t, "Previous Selection (first)", statusMsgs[0],
			"should wrap around when no previous selection exists")

		pagingArgs := rHub.getPagingArgs()
		// Expect two paging calls: ToScrollFirstItem, then JumpToLineRequest(lastLine)
		require.Len(t, pagingArgs, 2, "expected 2 paging args")

		jlr, ok := pagingArgs[1].(hub.JumpToLineRequest)
		require.True(t, ok, "second paging arg should be JumpToLineRequest")

		// The bug: lastLine stays at math.MaxUint64 instead of being updated to 40.
		// JumpToLineRequest is int, so MaxUint64 wraps to -1 on 64-bit.
		require.True(t, jlr.Line() >= 0,
			"lastLine must not be negative (math.MaxUint64 cast to int), got %d", jlr.Line())
		require.Equal(t, 40, jlr.Line(),
			"should jump to the last selected line (ID=40)")
	})

	t.Run("previous selection found", func(t *testing.T) {
		// Position cursor at line index 4 (ID=50), which is after both
		// selected lines. Should find previous selection at ID=40.
		state.Location().SetLineNumber(4)
		rHub.reset()

		doGoToPreviousSelection(ctx, state, Event{})

		statusMsgs := rHub.getStatusMsgs()
		require.NotEmpty(t, statusMsgs, "should have sent a status message")
		require.Equal(t, "Previous Selection", statusMsgs[0])

		pagingArgs := rHub.getPagingArgs()
		require.Len(t, pagingArgs, 2, "expected 2 paging args")

		jlr, ok := pagingArgs[1].(hub.JumpToLineRequest)
		require.True(t, ok, "second paging arg should be JumpToLineRequest")
		require.Equal(t, 40, jlr.Line(),
			"should jump to the previous selected line (ID=40)")
	})

	t.Run("skips to nearest previous selection", func(t *testing.T) {
		// Position cursor at line index 3 (ID=40). The previous selection
		// should be ID=20, not ID=40 (since 40 is not < 40).
		state.Location().SetLineNumber(3)
		rHub.reset()

		doGoToPreviousSelection(ctx, state, Event{})

		statusMsgs := rHub.getStatusMsgs()
		require.NotEmpty(t, statusMsgs, "should have sent a status message")
		require.Equal(t, "Previous Selection", statusMsgs[0])

		pagingArgs := rHub.getPagingArgs()
		require.Len(t, pagingArgs, 2, "expected 2 paging args")

		jlr, ok := pagingArgs[1].(hub.JumpToLineRequest)
		require.True(t, ok, "second paging arg should be JumpToLineRequest")
		require.Equal(t, 20, jlr.Line(),
			"should jump to ID=20, the nearest previous selected line")
	})
}

func TestNextSelectionNavigation(t *testing.T) {
	ctx := context.Background()

	lines := []line.Line{
		line.NewRaw(10, "line-10", false, false),
		line.NewRaw(20, "line-20", false, false),
		line.NewRaw(30, "line-30", false, false),
		line.NewRaw(40, "line-40", false, false),
		line.NewRaw(50, "line-50", false, false),
	}

	mb := NewMemoryBuffer(0)
	mb.lines = lines

	rHub := &recordingHub{}

	state := New()
	state.hub = rHub
	state.selection = NewSelection()
	state.currentLineBuffer = mb

	state.Selection().Add(lines[1]) // ID=20
	state.Selection().Add(lines[3]) // ID=40

	t.Run("next selection found", func(t *testing.T) {
		// Cursor at line index 0 (ID=10). Next selection should be ID=20.
		state.Location().SetLineNumber(0)
		rHub.reset()

		doGoToNextSelection(ctx, state, Event{})

		statusMsgs := rHub.getStatusMsgs()
		require.NotEmpty(t, statusMsgs)
		require.Equal(t, "Next Selection", statusMsgs[0])

		pagingArgs := rHub.getPagingArgs()
		require.Len(t, pagingArgs, 2)

		jlr, ok := pagingArgs[1].(hub.JumpToLineRequest)
		require.True(t, ok)
		require.Equal(t, 20, jlr.Line(),
			"should jump to the next selected line (ID=20)")
	})

	t.Run("skips to nearest next selection", func(t *testing.T) {
		// Cursor at line index 1 (ID=20). Next selection should be ID=40.
		state.Location().SetLineNumber(1)
		rHub.reset()

		doGoToNextSelection(ctx, state, Event{})

		statusMsgs := rHub.getStatusMsgs()
		require.NotEmpty(t, statusMsgs)
		require.Equal(t, "Next Selection", statusMsgs[0])

		pagingArgs := rHub.getPagingArgs()
		require.Len(t, pagingArgs, 2)

		jlr, ok := pagingArgs[1].(hub.JumpToLineRequest)
		require.True(t, ok)
		require.Equal(t, 40, jlr.Line(),
			"should jump to the next selected line (ID=40)")
	})

	t.Run("wrap around to first selected line", func(t *testing.T) {
		// Cursor at line index 4 (ID=50), past all selections.
		// Should wrap around to the first selected line (ID=20).
		state.Location().SetLineNumber(4)
		rHub.reset()

		doGoToNextSelection(ctx, state, Event{})

		statusMsgs := rHub.getStatusMsgs()
		require.NotEmpty(t, statusMsgs)
		require.Equal(t, "Next Selection (first)", statusMsgs[0],
			"should wrap around when no next selection exists")

		pagingArgs := rHub.getPagingArgs()
		require.Len(t, pagingArgs, 2)

		jlr, ok := pagingArgs[1].(hub.JumpToLineRequest)
		require.True(t, ok)
		require.Equal(t, 20, jlr.Line(),
			"should wrap to the first selected line (ID=20)")
	})
}

func TestGHIssue428_PgUpPgDnDefaultBindings(t *testing.T) {
	// Issue #428: PgUp/PgDn keys should be bound by default to
	// ScrollPageUp/ScrollPageDown, just like Home/End are bound
	// to ScrollFirstItem/ScrollLastItem.

	ctx := context.Background()
	rHub := &recordingHub{}

	state := New()
	state.hub = rHub
	state.selection = NewSelection()
	state.currentLineBuffer = NewMemoryBuffer(0)

	// Populate the keymap with defaults (no custom config).
	state.config.Keymap = map[string]string{}
	state.config.Action = map[string][]string{}
	require.NoError(t, state.populateKeymap(), "populateKeymap should succeed")

	km := state.Keymap()

	t.Run("PgDn triggers ScrollPageDown", func(t *testing.T) {
		rHub.reset()

		ev := Event{Key: keyseq.KeyPgdn}
		err := km.ExecuteAction(ctx, state, ev)
		require.NoError(t, err, "PgDn should resolve to an action")

		pagingArgs := rHub.getPagingArgs()
		require.Len(t, pagingArgs, 1, "expected one paging call")
		require.Equal(t, hub.ToScrollPageDown, pagingArgs[0],
			"PgDn should trigger ScrollPageDown")
	})

	t.Run("PgUp triggers ScrollPageUp", func(t *testing.T) {
		rHub.reset()

		ev := Event{Key: keyseq.KeyPgup}
		err := km.ExecuteAction(ctx, state, ev)
		require.NoError(t, err, "PgUp should resolve to an action")

		pagingArgs := rHub.getPagingArgs()
		require.Len(t, pagingArgs, 1, "expected one paging call")
		require.Equal(t, hub.ToScrollPageUp, pagingArgs[0],
			"PgUp should trigger ScrollPageUp")
	})
}

// TestGHIssue455_RefreshScreenSendsForceSync verifies that doRefreshScreen
// sends DrawOptions with both DisableCache and ForceSync set to true.
func TestGHIssue455_RefreshScreenSendsForceSync(t *testing.T) {
	ctx := context.Background()
	rHub := &recordingHub{}

	state := New()
	state.hub = rHub
	state.selection = NewSelection()
	state.currentLineBuffer = NewMemoryBuffer(0)

	doRefreshScreen(ctx, state, Event{})

	drawArgs := rHub.getDrawArgs()
	require.Len(t, drawArgs, 1, "expected exactly 1 SendDraw call")

	opts := drawArgs[0]
	require.NotNil(t, opts, "SendDraw argument should not be nil")
	require.True(t, opts.DisableCache, "DisableCache should be true")
	require.True(t, opts.ForceSync, "ForceSync should be true for screen refresh")
}

func TestDoFreezeResults(t *testing.T) {
	ctx := context.Background()

	makeLines := func(values ...string) []line.Line {
		lines := make([]line.Line, len(values))
		for i, v := range values {
			lines[i] = line.NewRaw(uint64(i), v, false, false)
		}
		return lines
	}

	t.Run("freeze captures current buffer", func(t *testing.T) {
		rHub := &recordingHub{}
		state := New()
		state.hub = rHub
		state.selection = NewSelection()

		lines := makeLines("alpha", "beta", "gamma")
		mb := NewMemoryBuffer(0)
		mb.lines = lines
		state.currentLineBuffer = mb

		state.Query().Set("test")
		state.Caret().SetPos(4)

		doFreezeResults(ctx, state, Event{})

		fs := state.Frozen().Source()
		require.NotNil(t, fs, "frozenSource should be set")
		require.Equal(t, 3, fs.Size(), "frozen buffer should have 3 lines")

		for i, expected := range []string{"alpha", "beta", "gamma"} {
			l, err := fs.LineAt(i)
			require.NoError(t, err)
			require.Equal(t, expected, l.Buffer())
		}

		require.Equal(t, 0, state.Query().Len(), "query should be cleared")
		require.Equal(t, 0, state.Caret().Pos(), "caret should be at 0")

		statusMsgs := rHub.getStatusMsgs()
		require.Contains(t, statusMsgs, "Results frozen")
	})

	t.Run("ResetCurrentLineBuffer uses frozen source", func(t *testing.T) {
		rHub := &recordingHub{}
		state := New()
		state.hub = rHub
		state.selection = NewSelection()
		state.source = &Source{}

		lines := makeLines("frozen1", "frozen2")
		frozen := NewMemoryBuffer(0)
		frozen.lines = lines
		frozen.MarkComplete()
		state.Frozen().Set(frozen)

		state.ResetCurrentLineBuffer(context.Background())

		buf := state.CurrentLineBuffer()
		require.Equal(t, 2, buf.Size(), "should use frozen source")
		l, err := buf.LineAt(0)
		require.NoError(t, err)
		require.Equal(t, "frozen1", l.Buffer())
	})

	t.Run("unfreeze reverts to original source", func(t *testing.T) {
		rHub := &recordingHub{}
		state := New()
		state.hub = rHub
		state.selection = NewSelection()

		origLines := makeLines("orig1", "orig2", "orig3")
		origSource := &Source{}
		origSource.lines = origLines
		state.source = origSource

		frozen := NewMemoryBuffer(0)
		frozen.lines = makeLines("frozen1")
		frozen.MarkComplete()
		state.Frozen().Set(frozen)
		state.currentLineBuffer = frozen

		state.Query().Set("test")
		state.Caret().SetPos(4)

		doUnfreezeResults(ctx, state, Event{})

		require.Nil(t, state.Frozen().Source(), "frozenSource should be nil")
		require.Equal(t, 0, state.Query().Len(), "query should be cleared")
		require.Equal(t, 0, state.Caret().Pos(), "caret should be at 0")

		statusMsgs := rHub.getStatusMsgs()
		require.Contains(t, statusMsgs, "Results unfrozen")
	})

	t.Run("freeze with empty buffer does nothing", func(t *testing.T) {
		rHub := &recordingHub{}
		state := New()
		state.hub = rHub
		state.selection = NewSelection()
		state.currentLineBuffer = NewMemoryBuffer(0)

		doFreezeResults(ctx, state, Event{})

		require.Nil(t, state.Frozen().Source(), "frozenSource should not be set")

		statusMsgs := rHub.getStatusMsgs()
		require.Contains(t, statusMsgs, "Nothing to freeze")
	})

	t.Run("unfreeze when not frozen does nothing", func(t *testing.T) {
		rHub := &recordingHub{}
		state := New()
		state.hub = rHub
		state.selection = NewSelection()

		doUnfreezeResults(ctx, state, Event{})

		require.Nil(t, state.Frozen().Source(), "frozenSource should remain nil")

		statusMsgs := rHub.getStatusMsgs()
		require.Contains(t, statusMsgs, "No frozen results")
	})
}

func TestContextBuffer(t *testing.T) {
	// Helper: create source lines with sequential IDs (0-based, matching line.ID())
	makeSource := func(n int) *MemoryBuffer {
		mb := NewMemoryBuffer(n)
		for i := range n {
			mb.lines = append(mb.lines, line.NewRaw(uint64(i), fmt.Sprintf("line-%d", i), false, false))
		}
		return mb
	}

	// Helper: create a filtered buffer with specific source indices as matches
	makeFiltered := func(source *MemoryBuffer, indices []int) *MemoryBuffer {
		mb := NewMemoryBuffer(len(indices))
		for _, idx := range indices {
			l, _ := source.LineAt(idx)
			mb.lines = append(mb.lines, l)
		}
		return mb
	}

	t.Run("single match in middle", func(t *testing.T) {
		source := makeSource(10)
		filtered := makeFiltered(source, []int{5})

		cb := NewContextBuffer(filtered, source, 2)

		// Should have lines 3,4,5,6,7 (5 entries: 2 context before, match, 2 context after)
		require.Equal(t, 5, cb.Size())

		// Lines 3,4 should be ContextLine
		l0, _ := cb.LineAt(0)
		_, isCtx0 := l0.(*ContextLine)
		require.True(t, isCtx0, "line 0 should be ContextLine")
		require.Equal(t, uint64(3), l0.ID())

		l1, _ := cb.LineAt(1)
		_, isCtx1 := l1.(*ContextLine)
		require.True(t, isCtx1, "line 1 should be ContextLine")
		require.Equal(t, uint64(4), l1.ID())

		// Line 5 should be the matched line (not ContextLine)
		l2, _ := cb.LineAt(2)
		_, isCtx2 := l2.(*ContextLine)
		require.False(t, isCtx2, "line 2 should be the matched line, not ContextLine")
		require.Equal(t, uint64(5), l2.ID())

		// Lines 6,7 should be ContextLine
		l3, _ := cb.LineAt(3)
		_, isCtx3 := l3.(*ContextLine)
		require.True(t, isCtx3, "line 3 should be ContextLine")

		l4, _ := cb.LineAt(4)
		_, isCtx4 := l4.(*ContextLine)
		require.True(t, isCtx4, "line 4 should be ContextLine")

		// matchEntryIndices: filtered index 0 -> entry index 2
		require.Equal(t, 2, cb.MatchEntryIndices()[0])
	})

	t.Run("overlapping context merges", func(t *testing.T) {
		source := makeSource(10)
		// Two matches close together: indices 3 and 5 with context=2
		// Ranges: [1,5] and [3,7] -> merged: [1,7]
		filtered := makeFiltered(source, []int{3, 5})

		cb := NewContextBuffer(filtered, source, 2)

		// Should have lines 1,2,3,4,5,6,7 (7 entries)
		require.Equal(t, 7, cb.Size())

		// Check matched lines are not ContextLine
		l2, _ := cb.LineAt(2) // source index 3
		_, isCtx := l2.(*ContextLine)
		require.False(t, isCtx, "matched line at source index 3 should not be ContextLine")
		require.Equal(t, uint64(3), l2.ID())

		l4, _ := cb.LineAt(4) // source index 5
		_, isCtx2 := l4.(*ContextLine)
		require.False(t, isCtx2, "matched line at source index 5 should not be ContextLine")
		require.Equal(t, uint64(5), l4.ID())

		// matchEntryIndices: filtered 0 -> entry 2, filtered 1 -> entry 4
		require.Equal(t, 2, cb.MatchEntryIndices()[0])
		require.Equal(t, 4, cb.MatchEntryIndices()[1])
	})

	t.Run("match at boundary", func(t *testing.T) {
		source := makeSource(5)
		// Match at index 0 with context=3 -> range [0, 3] (clamped start)
		filtered := makeFiltered(source, []int{0})

		cb := NewContextBuffer(filtered, source, 3)

		// Should have lines 0,1,2,3 (4 entries)
		require.Equal(t, 4, cb.Size())

		// First line should be the match (not context)
		l0, _ := cb.LineAt(0)
		_, isCtx := l0.(*ContextLine)
		require.False(t, isCtx, "line 0 should be matched, not context")
		require.Equal(t, uint64(0), l0.ID())

		// Lines 1-3 should be context
		for i := 1; i < 4; i++ {
			l, _ := cb.LineAt(i)
			_, isCtx := l.(*ContextLine)
			require.True(t, isCtx, "line %d should be ContextLine", i)
		}
	})

	t.Run("match at end boundary", func(t *testing.T) {
		source := makeSource(5)
		// Match at index 4 (last) with context=3 -> range [1, 4] (clamped end)
		filtered := makeFiltered(source, []int{4})

		cb := NewContextBuffer(filtered, source, 3)

		// Should have lines 1,2,3,4 (4 entries)
		require.Equal(t, 4, cb.Size())

		// Last line should be the match
		l3, _ := cb.LineAt(3)
		_, isCtx := l3.(*ContextLine)
		require.False(t, isCtx, "last line should be matched, not context")
		require.Equal(t, uint64(4), l3.ID())
	})

	t.Run("empty filtered buffer", func(t *testing.T) {
		source := makeSource(10)
		filtered := NewMemoryBuffer(0)

		cb := NewContextBuffer(filtered, source, 3)

		require.Equal(t, 0, cb.Size())
	})
}

func TestDoZoomInOut(t *testing.T) {
	ctx := context.Background()

	// Build a source with 10 lines (IDs 0-9)
	makeState := func() (*Peco, *recordingHub, *MemoryBuffer) {
		source := NewMemoryBuffer(10)
		for i := range 10 {
			source.lines = append(source.lines, line.NewRaw(uint64(i), fmt.Sprintf("line-%d", i), false, false))
		}

		// Filtered buffer: matches at indices 3 and 7
		filtered := NewMemoryBuffer(2)
		l3, _ := source.LineAt(3)
		l7, _ := source.LineAt(7)
		filtered.lines = append(filtered.lines, l3, l7)

		rHub := &recordingHub{}
		state := New()
		state.hub = rHub
		state.selection = NewSelection()
		state.source = &Source{}
		state.source.lines = source.lines
		state.currentLineBuffer = filtered

		return state, rHub, filtered
	}

	t.Run("ZoomIn with filtered results", func(t *testing.T) {
		state, rHub, filtered := makeState()
		state.Location().SetLineNumber(0) // cursor on first match

		doZoomIn(ctx, state, Event{})

		// Should have set a context buffer
		buf := state.CurrentLineBuffer()
		_, isCtx := buf.(*ContextBuffer)
		require.True(t, isCtx, "current buffer should be ContextBuffer after ZoomIn")

		// Pre-zoom state should be saved
		require.Equal(t, filtered, state.Zoom().Buffer(), "preZoomBuffer should be the filtered buffer")
		require.Equal(t, 0, state.Zoom().LineNo(), "preZoomLineNo should be 0")

		// Should have sent a draw
		drawArgs := rHub.getDrawArgs()
		require.NotEmpty(t, drawArgs, "should have sent a draw")

		// Context buffer should have entries around matches 3 and 7
		ctxBuf := buf.(*ContextBuffer)
		require.True(t, ctxBuf.Size() > 2, "context buffer should have more entries than just matches")
	})

	t.Run("ZoomOut restores state", func(t *testing.T) {
		state, rHub, filtered := makeState()
		state.Location().SetLineNumber(0)

		// ZoomIn first
		doZoomIn(ctx, state, Event{})
		rHub.reset()

		// ZoomOut
		doZoomOut(ctx, state, Event{})

		// Buffer should be restored
		require.Equal(t, filtered, state.CurrentLineBuffer(), "buffer should be restored to filtered")

		// Cursor should be restored
		require.Equal(t, 0, state.Location().LineNumber(), "cursor should be restored")

		// Pre-zoom state should be cleared
		require.Nil(t, state.Zoom().Buffer(), "preZoomBuffer should be nil after ZoomOut")

		// Should have sent a draw
		drawArgs := rHub.getDrawArgs()
		require.NotEmpty(t, drawArgs, "should have sent a draw")
	})

	t.Run("ZoomIn when not filtered (source buffer)", func(t *testing.T) {
		state, rHub, _ := makeState()
		// Set current buffer to source
		state.currentLineBuffer = state.source

		doZoomIn(ctx, state, Event{})

		// Should be a no-op with status message
		statusMsgs := rHub.getStatusMsgs()
		require.NotEmpty(t, statusMsgs)
		require.Equal(t, "Nothing to zoom into", statusMsgs[0])

		// PreZoom should not be set
		require.Nil(t, state.Zoom().Buffer())
	})

	t.Run("ZoomOut when not zoomed", func(t *testing.T) {
		state, rHub, _ := makeState()

		doZoomOut(ctx, state, Event{})

		statusMsgs := rHub.getStatusMsgs()
		require.NotEmpty(t, statusMsgs)
		require.Equal(t, "Not zoomed in", statusMsgs[0])
	})

	t.Run("ZoomIn when already zoomed", func(t *testing.T) {
		state, rHub, _ := makeState()
		state.Location().SetLineNumber(0)

		// ZoomIn first
		doZoomIn(ctx, state, Event{})
		rHub.reset()

		// ZoomIn again
		doZoomIn(ctx, state, Event{})

		statusMsgs := rHub.getStatusMsgs()
		require.NotEmpty(t, statusMsgs)
		require.Equal(t, "Already zoomed in", statusMsgs[0])
	})
}
