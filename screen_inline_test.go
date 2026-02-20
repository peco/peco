package peco

import (
	"bytes"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/peco/peco/config"
	"github.com/stretchr/testify/require"
)

// newTestInlineScreen creates an InlineScreen backed by a SimulationScreen
// for testing coordinate translation and size behavior.
func newTestInlineScreen(termWidth, termHeight, inlineHeight int) (*InlineScreen, tcell.SimulationScreen) {
	sim := tcell.NewSimulationScreen("")
	sim.Init()
	sim.SetSize(termWidth, termHeight)

	s := &InlineScreen{
		heightSpec: config.HeightSpec{Value: inlineHeight, IsPercent: false},
		screen:     sim,
		height:     inlineHeight,
		yOffset:    termHeight - inlineHeight,
	}
	return s, sim
}

func TestInlineScreenSize(t *testing.T) {
	s, _ := newTestInlineScreen(80, 24, 10)
	defer s.screen.Fini()

	w, h := s.Size()
	require.Equal(t, 80, w)
	require.Equal(t, 10, h)
}

func TestInlineScreenSetCell(t *testing.T) {
	s, sim := newTestInlineScreen(80, 24, 10)
	defer s.screen.Fini()

	// SetCell at virtual y=0 should map to physical y=14 (24-10)
	s.SetCell(5, 0, 'A', config.ColorDefault, config.ColorDefault)
	s.Flush()

	// Read back from simulation screen at physical coordinates
	str, _, _ := sim.Get(5, 14)
	require.Equal(t, "A", str)

	// SetCell at virtual y=9 (last line) should map to physical y=23
	s.SetCell(10, 9, 'Z', config.ColorDefault, config.ColorDefault)
	s.Flush()

	str, _, _ = sim.Get(10, 23)
	require.Equal(t, "Z", str)
}

func TestInlineScreenSetCursor(t *testing.T) {
	s, sim := newTestInlineScreen(80, 24, 10)
	defer s.screen.Fini()

	// SetCursor at virtual (3, 2) should map to physical (3, 16)
	s.SetCursor(3, 2)
	s.Flush()

	cx, cy, visible := sim.GetCursor()
	require.True(t, visible)
	require.Equal(t, 3, cx)
	require.Equal(t, 16, cy) // 2 + (24-10) = 16
}

// TestInlineScreenPollEventLogsPanic verifies that when a panic occurs
// in the InlineScreen PollEvent goroutine, it is logged to errWriter
// rather than being silently swallowed (CODE_REVIEW.md §3.5).
func TestInlineScreenPollEventLogsPanic(t *testing.T) {
	var buf bytes.Buffer

	sim := tcell.NewSimulationScreen("")
	sim.Init()

	s := &InlineScreen{
		screen:    &panickingScreen{Screen: sim},
		errWriter: &buf,
		height:    10,
		yOffset:   14,
	}

	ctx := t.Context()

	evCh := s.PollEvent(ctx, nil)

	// The goroutine should panic, log it, and close the channel.
	select {
	case _, ok := <-evCh:
		require.False(t, ok, "expected channel to be closed after panic")
	case <-time.After(2 * time.Second):
		require.Fail(t, "PollEvent channel was not closed after panic")
	}

	// Verify that the panic was logged (not silently swallowed).
	output := buf.String()
	require.Contains(t, output, "peco: panic in PollEvent goroutine")
	require.Contains(t, output, "test: deliberate panic in PollEvent")
}

func TestInlineScreenNilSafety(t *testing.T) {
	s := &InlineScreen{}

	// All operations should be safe on a nil screen
	w, h := s.Size()
	require.Equal(t, 0, w)
	require.Equal(t, 0, h)

	s.SetCell(0, 0, 'X', config.ColorDefault, config.ColorDefault)
	s.SetCursor(0, 0)
	require.NoError(t, s.Flush())
}
