package peco

import (
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

func TestActionFunc(t *testing.T) {
	called := 0
	af := ActionFunc(func(_ *Input, _ termbox.Event) {
		called++
	})
	af.Execute(nil, termbox.Event{})
	if called != 1 {
		t.Errorf("Expected ActionFunc to be called once, but it got called %d times", called)
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

func expectCaretPos(t *testing.T, c interface {
	CaretPos() int
}, expect int) bool {
	if c.CaretPos() != expect {
		t.Errorf("Expected caret position %d, got %d", expect, c.CaretPos())
		return false
	}
	return true
}

func expectQueryString(t *testing.T, c interface {
	QueryString() string
}, expect string) bool {
	if c.QueryString() != expect {
		t.Errorf("Expected '%s', got '%s'", expect, c.QueryString())
		return false
	}
	return true
}

func TestDoDeleteForwardChar(t *testing.T) {
	ctx := NewCtx(nil)
	input := ctx.NewInput()

	ctx.SetQuery([]rune("Hello, World!"))
	ctx.SetCaretPos(5)
	doDeleteForwardChar(input, termbox.Event{})

	expectQueryString(t, ctx, "Hello World!")
	expectCaretPos(t, ctx, 5)

	ctx.SetCaretPos(runewidth.StringWidth(ctx.QueryString()))
	doDeleteForwardChar(input, termbox.Event{})

	expectQueryString(t, ctx, "Hello World!")
	expectCaretPos(t, ctx, runewidth.StringWidth(ctx.QueryString()))

	ctx.SetCaretPos(0)
	doDeleteForwardChar(input, termbox.Event{})

	expectQueryString(t, ctx, "ello World!")
	expectCaretPos(t, ctx, 0)
}

func TestDoDeleteForwardWord(t *testing.T) {
	ctx := NewCtx(nil)
	input := ctx.NewInput()

	ctx.SetQuery([]rune("Hello, World!"))
	ctx.SetCaretPos(5)
	doDeleteForwardWord(input, termbox.Event{})

	expectQueryString(t, ctx, "Hello World!")
	expectCaretPos(t, ctx, 5)

	ctx.SetCaretPos(runewidth.StringWidth(ctx.QueryString()))
	doDeleteForwardWord(input, termbox.Event{})

	expectQueryString(t, ctx, "Hello World!")
	expectCaretPos(t, ctx, runewidth.StringWidth(ctx.QueryString()))

	ctx.SetCaretPos(0)
	doDeleteForwardWord(input, termbox.Event{})

	expectQueryString(t, ctx, " World!")
	expectCaretPos(t, ctx, 0)

	ctx.SetCaretPos(1)
	doDeleteForwardWord(input, termbox.Event{})

	expectQueryString(t, ctx, " ")
}

func TestDoDeleteBackwardChar(t *testing.T) {
	ctx := NewCtx(nil)
	input := ctx.NewInput()

	ctx.SetQuery([]rune("Hello, World!"))
	ctx.SetCaretPos(5)
	doDeleteBackwardChar(input, termbox.Event{})

	expectQueryString(t, ctx, "Hell, World!")
	expectCaretPos(t, ctx, 4)

	ctx.SetCaretPos(runewidth.StringWidth(ctx.QueryString()))
	doDeleteBackwardChar(input, termbox.Event{})

	expectQueryString(t, ctx, "Hell, World")
	expectCaretPos(t, ctx, runewidth.StringWidth(ctx.QueryString()))

	ctx.SetCaretPos(0)
	doDeleteBackwardChar(input, termbox.Event{})

	expectQueryString(t, ctx, "Hell, World")
	expectCaretPos(t, ctx, 0)
}

func TestDoDeleteBackwardWord(t *testing.T) {
	ctx := NewCtx(nil)
	input := ctx.NewInput()

	// In case of an overflow (bug)
	ctx.SetQuery([]rune("foo"))
	ctx.SetCaretPos(5)
	doDeleteBackwardWord(input, termbox.Event{})

	// https://github.com/peco/peco/pull/184#issuecomment-54026739

	// Case 1. " foo<caret>" -> " "
	ctx.SetQuery([]rune(" foo"))
	ctx.SetCaretPos(4)
	doDeleteBackwardWord(input, termbox.Event{})

	expectQueryString(t, ctx, " ")
	expectCaretPos(t, ctx, 1)

	// Case 2. "foo bar<caret>" -> "foo "
	ctx.SetQuery([]rune("foo bar"))
	ctx.SetCaretPos(7)
	doDeleteBackwardWord(input, termbox.Event{})

	expectQueryString(t, ctx, "foo ")
	expectCaretPos(t, ctx, 4)
}
