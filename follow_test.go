package peco

import (
	"fmt"
	"strings"
	"testing"

	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/line"
	"github.com/stretchr/testify/require"
)

// makeFollowBuffer builds a MemoryBuffer with n lines labelled "lineN".
func makeFollowBuffer(n int) *MemoryBuffer {
	mb := NewMemoryBuffer(0)
	for i := range n {
		mb.lines = append(mb.lines, line.NewRaw(uint64(i), fmt.Sprintf("line%d", i), false, false))
	}
	return mb
}

// readScreenRow returns the text rendered on row y of the simulation screen,
// trailing blanks trimmed.
func readScreenRow(t *testing.T, s *SimScreen, y int) string {
	t.Helper()
	cells, w, _ := s.screen.GetContents()
	var b strings.Builder
	for x := range w {
		c := cells[y*w+x]
		if len(c.Runes) == 0 || c.Runes[0] == 0 {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(c.Runes[0])
	}
	return strings.TrimRight(b.String(), " ")
}

func newFollowState(t *testing.T, buf Buffer) (*Peco, *SimScreen) {
	t.Helper()
	screen := NewDummyScreen()
	state := New()
	state.screen = screen
	state.Filters().Add(filter.NewIgnoreCase())
	state.currentLineBuffer = buf
	return state, screen
}

func TestWindowCrop(t *testing.T) {
	buf := makeFollowBuffer(100)

	t.Run("tail window", func(t *testing.T) {
		fb := WindowCrop{offset: 90, perPage: 10}.Crop(buf)
		require.Equal(t, 10, fb.Size())
		first, err := fb.LineAt(0)
		require.NoError(t, err)
		require.Equal(t, "line90", first.DisplayString())
		last, err := fb.LineAt(9)
		require.NoError(t, err)
		require.Equal(t, "line99", last.DisplayString())
	})

	t.Run("partial window smaller than perPage", func(t *testing.T) {
		small := makeFollowBuffer(5)
		fb := WindowCrop{offset: 0, perPage: 10}.Crop(small)
		require.Equal(t, 5, fb.Size())
	})

	t.Run("negative offset clamps to zero", func(t *testing.T) {
		fb := WindowCrop{offset: -5, perPage: 3}.Crop(buf)
		require.Equal(t, 3, fb.Size())
		first, err := fb.LineAt(0)
		require.NoError(t, err)
		require.Equal(t, "line0", first.DisplayString())
	})

	t.Run("offset past end yields empty", func(t *testing.T) {
		fb := WindowCrop{offset: 200, perPage: 10}.Crop(buf)
		require.Equal(t, 0, fb.Size())
	})
}

// TestGHIssue820_FollowKeepsNewestVisible verifies that with follow mode on,
// DrawScreen pins the viewport to the tail of the buffer so the newest line
// is rendered on the last visible row (like tail -f), regardless of page
// alignment.
func TestGHIssue820_FollowKeepsNewestVisible(t *testing.T) {
	buf := makeFollowBuffer(50)
	state, screen := newFollowState(t, buf)
	state.Follow().Set(true)

	// top-down-query-bottom is the layout from the issue: list anchored to
	// the top (row 0), query at the bottom.
	layout, err := TopDownQueryBottomLayout(state)
	require.NoError(t, err)

	layout.DrawScreen(state, nil)

	loc := state.Location()
	perPage := layout.linesPerPage()

	require.Equal(t, 49, loc.LineNumber(), "cursor should be pinned to the newest line")
	require.Equal(t, 50, loc.Total())
	require.Equal(t, max(50-perPage, 0), loc.Offset(),
		"offset should be a sliding window over the tail, not page-aligned")

	// The list area is anchored at row 0; the newest line sits on the last
	// visible row.
	lastRow := perPage - 1
	require.Equal(t, "line49", readScreenRow(t, screen, lastRow),
		"newest line should be on the bottom row of the list area")
}

// TestFollowDefaultShowsHead verifies that without follow mode the viewport
// stays at the head of the buffer (unchanged behavior).
func TestFollowDefaultShowsHead(t *testing.T) {
	buf := makeFollowBuffer(50)
	state, screen := newFollowState(t, buf)
	require.False(t, state.IsFollowing())

	layout, err := TopDownQueryBottomLayout(state)
	require.NoError(t, err)

	layout.DrawScreen(state, nil)

	loc := state.Location()
	require.Equal(t, 0, loc.LineNumber())
	require.Equal(t, 0, loc.Offset())
	require.Equal(t, "line0", readScreenRow(t, screen, 0),
		"oldest line should be on the top row when not following")
}

// TestFollowDisabledByManualScroll verifies that any manual vertical
// navigation turns follow mode off.
func TestFollowDisabledByManualScroll(t *testing.T) {
	buf := makeFollowBuffer(50)
	state, _ := newFollowState(t, buf)
	state.Follow().Set(true)

	layout, err := TopDownQueryBottomLayout(state)
	require.NoError(t, err)
	// Pin to the tail first.
	layout.DrawScreen(state, nil)
	require.True(t, state.IsFollowing())

	moved := layout.MovePage(state, hub.ToLineAbove)
	require.True(t, moved)
	require.False(t, state.IsFollowing(),
		"manual vertical scroll should disable follow mode")
}

// TestToggleFollowAction verifies the ToggleFollow action flips follow state.
func TestToggleFollowAction(t *testing.T) {
	state := New()
	state.hub = nullHub{}
	require.False(t, state.IsFollowing())

	doToggleFollow(t.Context(), state, Event{})
	require.True(t, state.IsFollowing(), "follow should be on after first toggle")

	doToggleFollow(t.Context(), state, Event{})
	require.False(t, state.IsFollowing(), "follow should be off after second toggle")
}

// TestFollowConfigAndFlag verifies that --follow and the Follow config field
// both enable follow mode, with the CLI flag taking precedence.
func TestFollowConfigAndFlag(t *testing.T) {
	t.Run("CLI flag", func(t *testing.T) {
		p := newPeco()
		require.NoError(t, p.ApplyConfig(CLIOptions{OptFollow: true}))
		require.True(t, p.IsFollowing())
	})

	t.Run("config field", func(t *testing.T) {
		p := newPeco()
		p.config.Follow = true
		require.NoError(t, p.ApplyConfig(CLIOptions{}))
		require.True(t, p.IsFollowing())
	})

	t.Run("default off", func(t *testing.T) {
		p := newPeco()
		require.NoError(t, p.ApplyConfig(CLIOptions{}))
		require.False(t, p.IsFollowing())
	})
}

// TestToggleFollowRegistered verifies the action is wired into the registry so
// it can be bound from a keymap.
func TestToggleFollowRegistered(t *testing.T) {
	_, ok := nameToActions["peco.ToggleFollow"]
	require.True(t, ok, "peco.ToggleFollow should be registered")
}
