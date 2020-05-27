package peco

import (
	"testing"
	"time"
	"unicode/utf8"

	"context"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/query"
	"github.com/peco/peco/ui"
	"github.com/stretchr/testify/assert"
)

func TestActionFunc(t *testing.T) {
	called := 0
	af := ActionFunc(func(_ context.Context, _ *Peco, _ termbox.Event) {
		called++
	})
	af.Execute(context.TODO(), nil, termbox.Event{})
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

func expectCaretPos(t *testing.T, c *ui.Caret, expect int) bool {
	return assert.Equal(t, expect, c.Pos(), "Expected caret position %d, got %d", expect, c.Pos())
}

func expectQueryString(t *testing.T, q *query.Query, expect string) bool {
	return assert.Equal(t, expect, q.String(), "Expected '%s', got '%s'", expect, q.String())
}

func TestDoDeleteForwardChar(t *testing.T) {
	state := newPeco()
	q := state.Query()
	c := state.Caret()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = state.Run(ctx) }()
	defer cancel()

	<-state.Ready()

	q.Set("Hello, World!")
	c.SetPos(5)

	doDeleteForwardChar(ctx, state, termbox.Event{})

	if !expectQueryString(t, q, "Hello World!") {
		return
	}
	if !expectCaretPos(t, c, 5) {
		return
	}

	c.SetPos(q.Len())
	doDeleteForwardChar(ctx, state, termbox.Event{})

	expectQueryString(t, q, "Hello World!")
	expectCaretPos(t, c, q.Len())

	c.SetPos(0)
	doDeleteForwardChar(ctx, state, termbox.Event{})

	expectQueryString(t, q, "ello World!")
	expectCaretPos(t, c, 0)
}

func TestDoDeleteForwardWord(t *testing.T) {
	state := newPeco()
	q := state.Query()
	c := state.Caret()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = state.Run(ctx) }()
	defer cancel()

	<-state.Ready()

	q.Set("Hello, World!")
	c.SetPos(5)

	// delete the comma
	doDeleteForwardWord(ctx, state, termbox.Event{})
	if !expectQueryString(t, q, "Hello World!") {
		return
	}

	if !expectCaretPos(t, c, 5) {
		return
	}

	// at the end of the query, should not delete anything
	c.SetPos(q.Len())
	doDeleteForwardWord(ctx, state, termbox.Event{})

	if !expectQueryString(t, q, "Hello World!") {
		return
	}
	if !expectCaretPos(t, c, q.Len()) {
		return
	}

	// back to the first column, should delete 'Hello'
	c.SetPos(0)
	doDeleteForwardWord(ctx, state, termbox.Event{})

	if !expectQueryString(t, q, " World!") {
		return
	}

	if !expectCaretPos(t, c, 0) {
		return
	}

	// should delete "World"
	c.SetPos(1)
	doDeleteForwardWord(ctx, state, termbox.Event{})

	if !expectQueryString(t, q, " ") {
		return
	}
}

func TestDoDeleteBackwardChar(t *testing.T) {
	state := newPeco()
	q := state.Query()
	c := state.Caret()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = state.Run(ctx) }()
	defer cancel()

	<-state.Ready()

	q.Set("Hello, World!")
	c.SetPos(5)

	doDeleteBackwardChar(ctx, state, termbox.Event{})

	expectQueryString(t, q, "Hell, World!")
	expectCaretPos(t, c, 4)

	c.SetPos(q.Len())
	doDeleteBackwardChar(ctx, state, termbox.Event{})

	expectQueryString(t, q, "Hell, World")
	expectCaretPos(t, c, q.Len())

	c.SetPos(0)
	doDeleteBackwardChar(ctx, state, termbox.Event{})

	expectQueryString(t, q, "Hell, World")
	expectCaretPos(t, c, 0)
}

func TestDoDeleteBackwardWord(t *testing.T) {
	state := newPeco()
	q := state.Query()
	c := state.Caret()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = state.Run(ctx) }()
	defer cancel()

	<-state.Ready()

	// In case of an overflow (bug)
	q.Set("foo")
	c.SetPos(5)
	doDeleteBackwardWord(ctx, state, termbox.Event{})

	// https://github.com/peco/peco/pull/184#issuecomment-54026739

	// Case 1. " foo<caret>" -> " "
	q.Set(" foo")
	c.SetPos(4)
	doDeleteBackwardWord(ctx, state, termbox.Event{})

	if !expectQueryString(t, q, " ") {
		return
	}

	if !expectCaretPos(t, c, 1) {
		return
	}

	// Case 2. "foo bar<caret>" -> "foo "
	q.Set("foo bar")
	c.SetPos(7)
	doDeleteBackwardWord(ctx, state, termbox.Event{})

	if !expectQueryString(t, q, "foo ") {
		return
	}

	if !expectCaretPos(t, c, 4) {
		return
	}
}

func writeQueryToPrompt(t *testing.T, screen ui.Screen, message string) {
	for str := message; true; {
		r, size := utf8.DecodeRuneInString(str)
		if r == utf8.RuneError {
			assert.Equal(t, 0, size, "when in error, we should have size == 0")
			return
		}

		if r == ' ' {
			screen.SendEvent(termbox.Event{Key: termbox.KeySpace})
		} else {
			screen.SendEvent(termbox.Event{Ch: r})
		}
		str = str[size:]
	}
}

func TestDoAcceptChar(t *testing.T) {
	state := newPeco()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = state.Run(ctx) }()
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
	go func() { _ = state.Run(ctx) }()
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
		state.screen.SendEvent(termbox.Event{Key: termbox.KeyCtrlR})

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
	go func() { _ = state.Run(ctx) }()
	defer cancel()

	<-state.Ready()

	message := "Hello, World!"
	writeQueryToPrompt(t, state.screen, message)
	state.screen.SendEvent(termbox.Event{Key: termbox.KeyCtrlA})

	time.Sleep(time.Second)
	if !assert.Equal(t, state.Caret().Pos(), 0, "Expected caret position to be 0, got %d", state.Caret().Pos()) {
		return
	}

	state.screen.SendEvent(termbox.Event{Key: termbox.KeyCtrlE})
	time.Sleep(time.Second)

	if !assert.Equal(t, state.Caret().Pos(), len(message), "Expected caret position to be %d, got %d", len(message), state.Caret().Pos()) {
		return
	}

}

func TestBackToInitialFilter(t *testing.T) {
	state := newPeco()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = state.Run(ctx) }()
	defer cancel()

	<-state.Ready()

	state.config.Keymap["C-q"] = "peco.BackToInitialFilter"
	if !assert.NoError(t, state.populateKeymap(), "populateKeymap expected to succeed") {
		return
	}

	if !assert.Equal(t, state.Filters().Index(), 0, "Expected filter to be at position 0, got %d", state.Filters().Index()) {
		return
	}

	state.screen.SendEvent(termbox.Event{Key: termbox.KeyCtrlR})
	time.Sleep(time.Second)
	if !assert.Equal(t, state.Filters().Index(), 1, "Expected filter to be at position 1, got %d", state.Filters().Index()) {
		return
	}

	state.screen.SendEvent(termbox.Event{Key: termbox.KeyCtrlQ})
	time.Sleep(time.Second)
	if !assert.Equal(t, state.Filters().Index(), 0, "Expected filter to be at position 0, got %d", state.Filters().Index()) {
		return
	}
}
