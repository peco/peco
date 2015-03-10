package peco

import (
	"testing"
	"time"
	"unicode/utf8"

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

func TestDoAcceptChar(t *testing.T) {
	_, guard := setDummyScreen()
	defer guard()

	ctx := newCtx(nil, 25)
	defer ctx.Stop()
	ctx.startInput()

	writeQueryToPrompt := func(message string) {
		for str := message; true; {
			r, size := utf8.DecodeRuneInString(str)
			if r == utf8.RuneError {
				if size == 0 {
					t.Logf("End of string reached")
					break
				}
				t.Errorf("Failed to decode run in string: %s", r)
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

	message := "Hello, World!"
	writeQueryToPrompt(message)
	time.Sleep(500 * time.Millisecond)

	if qs := ctx.QueryString(); qs != message {
		t.Errorf("Expected query to be populated as '%s', but got '%s'", message, qs)
	}

	ctx.MoveCaretPos(-6)
	writeQueryToPrompt("Cruel ")

	time.Sleep(500 * time.Millisecond)

	expected := "Hello, Cruel World!"
	if qs := ctx.QueryString(); qs != expected {
		t.Errorf("Expected query to be populated as '%s', but got '%s'", expected, qs)
	}
}

func TestRotateFilter(t *testing.T) {
	_, guard := setDummyScreen()
	defer guard()

	ctx := newCtx(nil, 25)
	defer ctx.Stop()

	size := ctx.filters.Size()
	if size < 2 {
		t.Errorf("Can't proceed testing, only have 1 filter registered")
	}

	ctx.startInput()

	var prev QueryFilterer
	first := ctx.Filter()
	prev = first
	for i := 0; i < size; i++ {
		screen.SendEvent(termbox.Event{Key: termbox.KeyCtrlR})

		time.Sleep(500 * time.Millisecond)
		f := ctx.Filter()
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
