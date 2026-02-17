package peco

import (
	"testing"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
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
		{LayoutTypeTopDownQueryBottom, true},
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

func TestScreenStatusBar(t *testing.T) {
	screen := NewDummyScreen()
	st, err := newScreenStatusBar(screen, AnchorBottom, 0, NewStyleSet())
	require.NoError(t, err)
	st.PrintStatus("Hello, World!", 0)

	events := screen.interceptor.events
	if l := len(events["Flush"]); l != 1 {
		t.Errorf("Expected 1 Flush event, got %d", l)
		return
	}
}

func TestNullStatusBar(t *testing.T) {
	screen := NewDummyScreen()
	var st StatusBar = nullStatusBar{}
	st.PrintStatus("Hello, World!", 0)

	events := screen.interceptor.events
	if l := len(events["Flush"]); l != 0 {
		t.Errorf("Expected 0 Flush events with nullStatusBar, got %d", l)
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

// TestGHIssue294_PromptStyleUsedForPromptPrefix verifies that UserPrompt.Draw
// uses the Prompt style (not Basic) when rendering the prompt prefix string.
func TestGHIssue294_PromptStyleUsedForPromptPrefix(t *testing.T) {
	styles := NewStyleSet()
	styles.Prompt.fg = ColorGreen | AttrBold
	styles.Prompt.bg = ColorBlue
	// Make sure Basic is different so we can distinguish them.
	styles.Basic.fg = ColorDefault
	styles.Basic.bg = ColorDefault

	screen := NewDummyScreen()
	prompt, err := NewUserPrompt(screen, AnchorTop, 0, "QUERY>", styles)
	require.NoError(t, err)

	state := New()
	state.screen = screen

	state.Filters().Add(filter.NewIgnoreCase())

	prompt.Draw(state)

	// Collect SetCell events for y=0 (the prompt row).
	events := screen.interceptor.events["SetCell"]

	promptStr := "QUERY>"
	promptLen := len(promptStr)
	require.True(t, len(events) >= promptLen,
		"expected at least %d SetCell events, got %d", promptLen, len(events))

	// The first promptLen cells should use the Prompt style colors.
	for i := range promptLen {
		ev := events[i]
		x := ev[0].(int)
		ch := ev[2].(rune)
		fg := ev[3].(Attribute)
		bg := ev[4].(Attribute)

		require.Equal(t, i, x, "expected x=%d", i)
		require.Equal(t, rune(promptStr[i]), ch, "expected character %c at position %d", promptStr[i], i)
		require.Equal(t, styles.Prompt.fg, fg,
			"cell at x=%d should use Prompt.fg, got %v", i, fg)
		require.Equal(t, styles.Prompt.bg, bg,
			"cell at x=%d should use Prompt.bg, got %v", i, bg)
	}

	// The cells after the prompt should NOT use the Prompt style —
	// they should use the Query style (for the query text area).
	if len(events) > promptLen {
		ev := events[promptLen]
		fg := ev[3].(Attribute)
		require.NotEqual(t, styles.Prompt.fg, fg,
			"cell after prompt should not use Prompt style")
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
		listArea, err := NewListArea(screen, AnchorTop, 0, true, styles)
		require.NoError(t, err)

		state := New()
		state.screen = screen

		mb := NewMemoryBuffer(0)
		raw := line.NewRaw(0, text, false, false)
		matched := line.NewMatched(raw, matches)
		mb.lines = append(mb.lines, matched)
		// Add a second line so we can set the cursor on it,
		// keeping line 0 in Basic (non-selected) style.
		mb.lines = append(mb.lines, line.NewRaw(1, "other", false, false))
		state.currentLineBuffer = mb

		loc := state.Location()
		loc.SetPage(1)
		loc.SetPerPage(10)
		loc.SetLineNumber(1) // select line 1, so line 0 uses Basic style

		listArea.Draw(state, nil, 10, &hub.DrawOptions{DisableCache: true})

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

// TestGHIssue455_DrawScreenForceSync verifies that BasicLayout.DrawScreen
// calls Sync() (full redraw) instead of Flush() (differential) when
// DrawOptions.ForceSync is true.
func TestGHIssue455_DrawScreenForceSync(t *testing.T) {
	setupState := func(t *testing.T) (*Peco, *SimScreen) {
		t.Helper()

		screen := NewDummyScreen()
		state := New()
		state.screen = screen
		state.Filters().Add(filter.NewIgnoreCase())

		mb := NewMemoryBuffer(0)
		mb.lines = append(mb.lines, line.NewRaw(0, "line one", false, false))
		state.currentLineBuffer = mb

		loc := state.Location()
		loc.SetPage(1)
		loc.SetPerPage(10)
		loc.SetLineNumber(0)

		return state, screen
	}

	t.Run("ForceSync true calls Sync instead of final Flush", func(t *testing.T) {
		state, screen := setupState(t)
		layout, err := NewDefaultLayout(state)
		require.NoError(t, err)

		screen.interceptor.reset()
		layout.DrawScreen(state, &hub.DrawOptions{DisableCache: true, ForceSync: true})

		syncEvents := screen.interceptor.events["Sync"]
		flushEvents := screen.interceptor.events["Flush"]

		require.Len(t, syncEvents, 1, "expected exactly 1 Sync call")
		// DrawPrompt internally calls Flush, but the final DrawScreen
		// Flush should be replaced by Sync.
		for i, ev := range screen.interceptor.events["Flush"] {
			t.Logf("Flush event %d: %v", i, ev)
		}
		for i, ev := range screen.interceptor.events["Sync"] {
			t.Logf("Sync event %d: %v", i, ev)
		}
		// The prompt's Flush still fires, but the final screen Flush
		// is replaced by Sync. So Flush count should be 1 less than
		// the non-ForceSync case.
		flushCountWithSync := len(flushEvents)

		// Compare against the non-ForceSync case
		screen.interceptor.reset()
		layout.DrawScreen(state, &hub.DrawOptions{DisableCache: true, ForceSync: false})
		flushCountWithout := len(screen.interceptor.events["Flush"])

		require.Equal(t, flushCountWithout-1, flushCountWithSync,
			"ForceSync should replace exactly one Flush call with Sync")
	})

	t.Run("ForceSync false does not call Sync", func(t *testing.T) {
		state, screen := setupState(t)
		layout, err := NewDefaultLayout(state)
		require.NoError(t, err)

		screen.interceptor.reset()
		layout.DrawScreen(state, &hub.DrawOptions{DisableCache: true, ForceSync: false})

		syncEvents := screen.interceptor.events["Sync"]
		require.Empty(t, syncEvents, "expected no Sync calls when ForceSync is false")
	})

	t.Run("nil options does not call Sync", func(t *testing.T) {
		state, screen := setupState(t)
		layout, err := NewDefaultLayout(state)
		require.NoError(t, err)

		screen.interceptor.reset()
		layout.DrawScreen(state, nil)

		syncEvents := screen.interceptor.events["Sync"]
		require.Empty(t, syncEvents, "expected no Sync calls with nil options")
	})
}

// TestNewLayout verifies the layout registry returns correct layout types.
func TestNewLayout(t *testing.T) {
	makeState := func() *Peco {
		state := New()
		state.screen = NewDummyScreen()
		state.Filters().Add(filter.NewIgnoreCase())
		return state
	}

	t.Run("top-down", func(t *testing.T) {
		state := makeState()
		layout, err := NewLayout(LayoutTypeTopDown, state)
		require.NoError(t, err)
		require.True(t, layout.SortTopDown(), "top-down layout should sort top-down")
		require.Equal(t, AnchorTop, layout.prompt.anchor, "top-down prompt should be anchored at top")
	})

	t.Run("bottom-up", func(t *testing.T) {
		state := makeState()
		layout, err := NewLayout(LayoutTypeBottomUp, state)
		require.NoError(t, err)
		require.False(t, layout.SortTopDown(), "bottom-up layout should not sort top-down")
		require.Equal(t, AnchorBottom, layout.prompt.anchor, "bottom-up prompt should be anchored at bottom")
	})

	t.Run("top-down-query-bottom", func(t *testing.T) {
		state := makeState()
		layout, err := NewLayout(LayoutTypeTopDownQueryBottom, state)
		require.NoError(t, err)
		require.True(t, layout.SortTopDown(), "top-down-query-bottom layout should sort top-down")
		require.Equal(t, AnchorBottom, layout.prompt.anchor, "top-down-query-bottom prompt should be anchored at bottom")
		require.Equal(t, AnchorTop, layout.list.anchor, "top-down-query-bottom list should be anchored at top")
	})

	t.Run("unknown falls back to top-down", func(t *testing.T) {
		state := makeState()
		layout, err := NewLayout("unknown-layout", state)
		require.NoError(t, err)
		require.True(t, layout.SortTopDown(), "fallback layout should sort top-down")
		require.Equal(t, AnchorTop, layout.prompt.anchor, "fallback prompt should be anchored at top")
	})
}

// TestTopDownQueryBottomLayout verifies the specific properties of the
// top-down-query-bottom layout.
func TestTopDownQueryBottomLayout(t *testing.T) {
	state := New()
	state.screen = NewDummyScreen()

	state.Filters().Add(filter.NewIgnoreCase())

	layout, err := NewTopDownQueryBottomLayout(state)
	require.NoError(t, err)

	require.Equal(t, AnchorBottom, layout.prompt.anchor,
		"prompt should be anchored at bottom")
	require.Equal(t, 1+extraOffset, layout.prompt.anchorOffset,
		"prompt anchor offset should be 1+extraOffset")
	require.Equal(t, AnchorTop, layout.list.anchor,
		"list should be anchored at top")
	require.Equal(t, 0, layout.list.anchorOffset,
		"list anchor offset should be 0")
	require.True(t, layout.list.sortTopDown,
		"list should sort top-down")
	require.True(t, layout.SortTopDown(),
		"SortTopDown() should return true")
}

// TestInvalidAnchorReturnsError verifies that invalid vertical anchors
// produce errors instead of panics.
func TestInvalidAnchorReturnsError(t *testing.T) {
	screen := NewDummyScreen()
	invalidAnchor := VerticalAnchor(99)

	t.Run("NewAnchorSettings", func(t *testing.T) {
		_, err := NewAnchorSettings(screen, invalidAnchor, 0)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid vertical anchor")
	})

	t.Run("NewUserPrompt", func(t *testing.T) {
		_, err := NewUserPrompt(screen, invalidAnchor, 0, "QUERY>", NewStyleSet())
		require.Error(t, err)
	})

	t.Run("NewListArea", func(t *testing.T) {
		_, err := NewListArea(screen, invalidAnchor, 0, true, NewStyleSet())
		require.Error(t, err)
	})

	t.Run("newScreenStatusBar", func(t *testing.T) {
		_, err := newScreenStatusBar(screen, invalidAnchor, 0, NewStyleSet())
		require.Error(t, err)
	})
}
