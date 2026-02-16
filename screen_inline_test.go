package peco

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/require"
)

// newTestInlineScreen creates an InlineScreen backed by a SimulationScreen
// for testing coordinate translation and size behavior.
func newTestInlineScreen(termWidth, termHeight, inlineHeight int) (*InlineScreen, tcell.SimulationScreen) {
	sim := tcell.NewSimulationScreen("")
	sim.Init()
	sim.SetSize(termWidth, termHeight)

	s := &InlineScreen{
		heightSpec: HeightSpec{Value: inlineHeight, IsPercent: false},
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
	s.SetCell(5, 0, 'A', ColorDefault, ColorDefault)
	s.Flush()

	// Read back from simulation screen at physical coordinates
	str, _, _ := sim.Get(5, 14)
	require.Equal(t, "A", str)

	// SetCell at virtual y=9 (last line) should map to physical y=23
	s.SetCell(10, 9, 'Z', ColorDefault, ColorDefault)
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

func TestInlineScreenNilSafety(t *testing.T) {
	s := &InlineScreen{}

	// All operations should be safe on a nil screen
	w, h := s.Size()
	require.Equal(t, 0, w)
	require.Equal(t, 0, h)

	s.SetCell(0, 0, 'X', ColorDefault, ColorDefault)
	s.SetCursor(0, 0)
	require.NoError(t, s.Flush())
}
