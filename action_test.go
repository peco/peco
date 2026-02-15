package peco

import (
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"context"

	"github.com/peco/peco/filter"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/line"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingHub wraps nullHub but records SendPaging, SendStatusMsg, and SendDraw calls.
type recordingHub struct {
	nullHub
	mu         sync.Mutex
	pagingArgs []interface{}
	statusMsgs []string
	drawArgs   []interface{}
}

func (h *recordingHub) SendPaging(_ context.Context, v interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pagingArgs = append(h.pagingArgs, v)
}

func (h *recordingHub) SendStatusMsg(_ context.Context, msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.statusMsgs = append(h.statusMsgs, msg)
}

func (h *recordingHub) SendDraw(_ context.Context, v interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.drawArgs = append(h.drawArgs, v)
}

func (h *recordingHub) getDrawArgs() []interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]interface{}, len(h.drawArgs))
	copy(dst, h.drawArgs)
	return dst
}

func (h *recordingHub) getPagingArgs() []interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	dst := make([]interface{}, len(h.pagingArgs))
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
	}
	for _, name := range names {
		if _, ok := nameToActions[name]; !ok {
			t.Errorf("Action %s should exist, but it does not", name)
		}
	}
}

func expectCaretPos(t *testing.T, c *Caret, expect int) bool {
	return assert.Equal(t, expect, c.Pos(), "Expected caret position %d, got %d", expect, c.Pos())
}

func expectQueryString(t *testing.T, q *Query, expect string) bool {
	return assert.Equal(t, expect, q.String(), "Expected '%s', got '%s'", expect, q.String())
}

func TestDoDeleteForwardChar(t *testing.T) {
	state := newPeco()
	q := state.Query()
	c := state.Caret()

	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

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
	state := newPeco()
	q := state.Query()
	c := state.Caret()

	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

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
	state := newPeco()
	q := state.Query()
	c := state.Caret()

	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

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
	state := newPeco()
	q := state.Query()
	c := state.Caret()

	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

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
	state := newPeco()

	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

	message := "Hello, World!"
	writeQueryToPrompt(t, state.screen, message)
	time.Sleep(500 * time.Millisecond)

	if qs := state.Query().String(); qs != message {
		t.Errorf("Expected query to be populated as '%s', but got '%s'", message, qs)
	}

	state.Caret().Move(-1 * len("World!"))
	writeQueryToPrompt(t, state.screen, "Cruel ")

	time.Sleep(500 * time.Millisecond)

	expected := "Hello, Cruel World!"
	if qs := state.Query().String(); qs != expected {
		t.Errorf("Expected query to be populated as '%s', but got '%s'", expected, qs)
	}
}

func TestRotateFilter(t *testing.T) {
	state := newPeco()

	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

	size := state.filters.Size()
	if size <= 1 {
		t.Skip("Can't proceed testing, only have 1 filter registered")
		return
	}

	var prev filter.Filter
	first := state.Filters().Current()
	prev = first
	for i := 0; i < size; i++ {
		state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlR})

		time.Sleep(500 * time.Millisecond)
		f := state.Filters().Current()
		if f == prev {
			t.Errorf("failed to rotate")
		}
		prev = f
	}

	if first != prev {
		t.Errorf("should have rotated back to first one, but didn't")
	}

	// TODO toggle ExecQuery()
}

func TestBeginningOfLineAndEndOfLine(t *testing.T) {
	state := newPeco()

	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

	message := "Hello, World!"
	writeQueryToPrompt(t, state.screen, message)
	state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlA})

	time.Sleep(time.Second)
	if !assert.Equal(t, state.Caret().Pos(), 0, "Expected caret position to be 0, got %d", state.Caret().Pos()) {
		return
	}

	state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlE})
	time.Sleep(time.Second)

	if !assert.Equal(t, state.Caret().Pos(), len(message), "Expected caret position to be %d, got %d", len(message), state.Caret().Pos()) {
		return
	}

}

func TestBackToInitialFilter(t *testing.T) {
	state := newPeco()

	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

	state.config.Keymap["C-q"] = "peco.BackToInitialFilter"
	if !assert.NoError(t, state.populateKeymap(), "populateKeymap expected to succeed") {
		return
	}

	if !assert.Equal(t, state.Filters().Index(), 0, "Expected filter to be at position 0, got %d", state.Filters().Index()) {
		return
	}

	state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlR})
	time.Sleep(time.Second)
	if !assert.Equal(t, state.Filters().Index(), 1, "Expected filter to be at position 1, got %d", state.Filters().Index()) {
		return
	}

	state.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyCtrlQ})
	time.Sleep(time.Second)
	if !assert.Equal(t, state.Filters().Index(), 0, "Expected filter to be at position 0, got %d", state.Filters().Index()) {
		return
	}
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
		line.NewRaw(10, "line-10", false),
		line.NewRaw(20, "line-20", false),
		line.NewRaw(30, "line-30", false),
		line.NewRaw(40, "line-40", false),
		line.NewRaw(50, "line-50", false),
	}

	// Build a MemoryBuffer containing those lines.
	mb := NewMemoryBuffer()
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

		jlr, ok := pagingArgs[1].(JumpToLineRequest)
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

		jlr, ok := pagingArgs[1].(JumpToLineRequest)
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

		jlr, ok := pagingArgs[1].(JumpToLineRequest)
		require.True(t, ok, "second paging arg should be JumpToLineRequest")
		require.Equal(t, 20, jlr.Line(),
			"should jump to ID=20, the nearest previous selected line")
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
	state.currentLineBuffer = NewMemoryBuffer()

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
		require.Equal(t, ToScrollPageDown, pagingArgs[0],
			"PgDn should trigger ScrollPageDown")
	})

	t.Run("PgUp triggers ScrollPageUp", func(t *testing.T) {
		rHub.reset()

		ev := Event{Key: keyseq.KeyPgup}
		err := km.ExecuteAction(ctx, state, ev)
		require.NoError(t, err, "PgUp should resolve to an action")

		pagingArgs := rHub.getPagingArgs()
		require.Len(t, pagingArgs, 1, "expected one paging call")
		require.Equal(t, ToScrollPageUp, pagingArgs[0],
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
	state.currentLineBuffer = NewMemoryBuffer()

	doRefreshScreen(ctx, state, Event{})

	drawArgs := rHub.getDrawArgs()
	require.Len(t, drawArgs, 1, "expected exactly 1 SendDraw call")

	opts, ok := drawArgs[0].(*DrawOptions)
	require.True(t, ok, "SendDraw argument should be *DrawOptions")
	require.True(t, opts.DisableCache, "DisableCache should be true")
	require.True(t, opts.ForceSync, "ForceSync should be true for screen refresh")
}
