package peco

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCaretInitialPos(t *testing.T) {
	t.Parallel()
	var c Caret
	require.Equal(t, 0, c.Pos())
}

func TestCaretSetPos(t *testing.T) {
	t.Parallel()
	var c Caret
	c.SetPos(5)
	require.Equal(t, 5, c.Pos())

	c.SetPos(0)
	require.Equal(t, 0, c.Pos())

	c.SetPos(100)
	require.Equal(t, 100, c.Pos())
}

func TestCaretMove(t *testing.T) {
	t.Parallel()
	var c Caret
	c.SetPos(5)

	c.Move(3)
	require.Equal(t, 8, c.Pos())

	c.Move(-2)
	require.Equal(t, 6, c.Pos())

	// Move to negative territory
	c.Move(-10)
	require.Equal(t, -4, c.Pos())
}

func TestCaretMoveFromZero(t *testing.T) {
	t.Parallel()
	var c Caret

	c.Move(1)
	require.Equal(t, 1, c.Pos())

	c.Move(-1)
	require.Equal(t, 0, c.Pos())
}

func TestCaretMultipleMoves(t *testing.T) {
	t.Parallel()
	var c Caret
	for range 10 {
		c.Move(1)
	}
	require.Equal(t, 10, c.Pos())

	for range 5 {
		c.Move(-1)
	}
	require.Equal(t, 5, c.Pos())
}
