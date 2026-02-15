package peco

import (
	"testing"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/peco/peco/line"
	"github.com/stretchr/testify/require"
)

func TestLayoutType(t *testing.T) {
	layouts := []struct {
		value    LayoutType
		expectOK bool
	}{
		{LayoutTypeTopDown, true},
		{LayoutTypeBottomUp, true},
		{"foobar", false},
	}
	for _, l := range layouts {
		valid := IsValidLayoutType(l.value)
		if valid != l.expectOK {
			t.Errorf("LayoutType %s, expected IsValidLayoutType to return %t, but got %t",
				l.value,
				l.expectOK,
				valid,
			)
		}
	}
}

func TestPrintScreen(t *testing.T) {
	screen := NewDummyScreen()

	makeVerifier := func(initX, initY int, fill bool) func(string) {
		return func(msg string) {
			screen.interceptor.reset()
			t.Logf("Checking printScreen(%d, %d, %s, %t)", initX, initY, msg, fill)
			width := utf8.RuneCountInString(msg)
			screen.Print(PrintArgs{
				X:    initX,
				Y:    initY,
				Fg:   ColorDefault,
				Bg:   ColorDefault,
				Msg:  msg,
				Fill: fill,
			})
			events := screen.interceptor.events["SetCell"]
			if !fill {
				if len(events) != width {
					t.Errorf("Expected %d SetCell events, got %d",
						width,
						len(events),
					)
				}
				return
			}

			// fill == true
			w, _ := screen.Size()
			if rw := runewidth.StringWidth(msg); rw != width {
				w -= rw - width
			}
			if len(events) != w {
				t.Errorf("Expected %d SetCell events, got %d",
					w,
					len(events),
				)
				return
			}
		}
	}

	verify := makeVerifier(0, 0, false)
	verify("Hello, World!")
	verify("日本語")

	verify = makeVerifier(0, 0, true)
	verify("Hello, World!")
	verify("日本語")
}

func TestStatusBar(t *testing.T) {
	screen := NewDummyScreen()
	st := NewStatusBar(screen, AnchorBottom, 0, NewStyleSet())
	st.PrintStatus("Hello, World!", 0)

	events := screen.interceptor.events
	if l := len(events["Flush"]); l != 1 {
		t.Errorf("Expected 1 Flush event, got %d", l)
		return
	}
}

func TestMergeAttribute(t *testing.T) {
	colors := stringToFg

	// merge colors
	tests := [][]string{
		{"red", "green", "yellow"},
		{"red", "blue", "magenta"},
		{"green", "blue", "cyan"},
		{"yellow", "blue", "white"},
		{"magenta", "green", "white"},
		{"cyan", "red", "white"},
		{"yellow", "magenta", "white"},
		{"magenta", "cyan", "white"},
		{"cyan", "yellow", "white"},
	}

	for _, c := range tests {
		if m := mergeAttribute(colors[c[0]], colors[c[1]]); m != colors[c[2]] {
			t.Errorf("(%s + %s) expected %d(%s), got %d", c[0], c[1], colors[c[2]], c[2], m)
		}
	}

	// merge with white
	for _, c := range colors {
		if m := mergeAttribute(c, colors["white"]); m != colors["white"] {
			t.Errorf("expected white(%d), got %d", colors["white"], m)
		}
	}

	// merge attributes
	if m := mergeAttribute(AttrBold|colors["red"], AttrUnderline|colors["cyan"]); m != AttrBold|AttrUnderline|colors["white"] {
		t.Errorf("expected %d, got %d", AttrBold|AttrUnderline|colors["white"], m)
	}

}

// TestGHIssue460_MatchedStyleDoesNotBleedToEndOfLine verifies that matched
// text highlighting in ListArea.Draw does not extend to the screen edge.
func TestGHIssue460_MatchedStyleDoesNotBleedToEndOfLine(t *testing.T) {
	// Use a distinct Matched.bg so we can detect it in SetCell events.
	styles := NewStyleSet()
	styles.Matched.bg = ColorBlue

	matchedBg := mergeAttribute(styles.Basic.bg, styles.Matched.bg) // ColorBlue
	basicBg := styles.Basic.bg                                      // ColorDefault

	// Helper: set up a Peco state with one matched line and draw it,
	// returning the SetCell events for the line's row (y=0).
	drawAndCollect := func(t *testing.T, text string, matches [][]int) []interceptorArgs {
		t.Helper()

		screen := NewDummyScreen()
		listArea := NewListArea(screen, AnchorTop, 0, true, styles)

		state := New()
		state.screen = screen
		state.skipReadConfig = true

		mb := NewMemoryBuffer()
		raw := line.NewRaw(0, text, false)
		matched := line.NewMatched(raw, matches)
		mb.lines = append(mb.lines, matched)
		// Add a second line so we can set the cursor on it,
		// keeping line 0 in Basic (non-selected) style.
		mb.lines = append(mb.lines, line.NewRaw(1, "other", false))
		state.currentLineBuffer = mb

		loc := state.Location()
		loc.SetPage(1)
		loc.SetPerPage(10)
		loc.SetLineNumber(1) // select line 1, so line 0 uses Basic style

		listArea.Draw(state, nil, 10, &DrawOptions{DisableCache: true})

		// Collect SetCell events for y=0 (our matched line).
		var row []interceptorArgs
		for _, ev := range screen.interceptor.events["SetCell"] {
			if ev[1].(int) == 0 {
				row = append(row, ev)
			}
		}
		return row
	}

	t.Run("match at end of line", func(t *testing.T) {
		// "hello world" with "world" matched at end [6,11].
		row := drawAndCollect(t, "hello world", [][]int{{6, 11}})

		screenWidth := 80
		require.Equal(t, screenWidth, len(row),
			"expected SetCell events to cover the full screen width")

		for _, ev := range row {
			x := ev[0].(int)
			bg := ev[4].(Attribute)
			if x >= 6 && x <= 10 {
				require.Equal(t, matchedBg, bg,
					"cell at x=%d should have matched bg", x)
			} else {
				require.Equal(t, basicBg, bg,
					"cell at x=%d should have basic bg, not matched bg", x)
			}
		}
	})

	t.Run("match in middle of line", func(t *testing.T) {
		// "hello world, goodbye" with "world" matched at [6,11].
		row := drawAndCollect(t, "hello world, goodbye", [][]int{{6, 11}})

		screenWidth := 80
		require.Equal(t, screenWidth, len(row),
			"expected SetCell events to cover the full screen width")

		for _, ev := range row {
			x := ev[0].(int)
			bg := ev[4].(Attribute)
			if x >= 6 && x <= 10 {
				require.Equal(t, matchedBg, bg,
					"cell at x=%d should have matched bg", x)
			} else {
				require.Equal(t, basicBg, bg,
					"cell at x=%d should have basic bg, not matched bg", x)
			}
		}
	})
}
