package peco

import (
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

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
	CaretPos() CaretPosition
}, expect CaretPosition) bool {
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
	expectCaretPos(t, ctx, CaretPosition(runewidth.StringWidth(ctx.QueryString())))

	ctx.SetCaretPos(0)
	doDeleteForwardChar(input, termbox.Event{})

	expectQueryString(t, ctx, "ello World!")
	expectCaretPos(t, ctx, 0)
}
