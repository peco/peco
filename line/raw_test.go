package line

import (
	"testing"

	"github.com/peco/peco/internal/ansi"
	"github.com/stretchr/testify/require"
)

func TestNewRaw_NoANSI(t *testing.T) {
	rl := NewRaw(1, "hello world", false, false)
	require.Equal(t, "hello world", rl.DisplayString())
	require.Nil(t, rl.ANSIAttrs())
	require.Equal(t, "hello world", rl.Output())
}

func TestNewRaw_ANSIEnabled_NoEscape(t *testing.T) {
	rl := NewRaw(1, "hello world", false, true)
	require.Equal(t, "hello world", rl.DisplayString())
	require.Nil(t, rl.ANSIAttrs())
	require.Equal(t, "hello world", rl.Output())
}

func TestNewRaw_ANSIEnabled_WithEscape(t *testing.T) {
	rl := NewRaw(1, "\x1b[31mRed\x1b[0m text", false, true)
	require.Equal(t, "Red text", rl.DisplayString())
	require.NotNil(t, rl.ANSIAttrs())
	require.Len(t, rl.ANSIAttrs(), 2)
	require.Equal(t, ansi.ColorRed, rl.ANSIAttrs()[0].Fg)
	require.Equal(t, 3, rl.ANSIAttrs()[0].Length)
	// Output preserves original ANSI codes
	require.Equal(t, "\x1b[31mRed\x1b[0m text", rl.Output())
}

func TestNewRaw_ANSIDisabled_WithEscape(t *testing.T) {
	rl := NewRaw(1, "\x1b[31mRed\x1b[0m text", false, false)
	// When ANSI is disabled, DisplayString still strips ANSI via StripANSISequence
	require.Equal(t, "Red text", rl.DisplayString())
	// But no parsed attributes
	require.Nil(t, rl.ANSIAttrs())
}

func TestNewRaw_ANSIEnabled_WithSep(t *testing.T) {
	rl := NewRaw(1, "\x1b[31mRed\x1b[0m display\x00output part", true, true)
	// Display should only show the part before \0, ANSI-stripped
	require.Equal(t, "Red display", rl.DisplayString())
	// Output should be the part after \0
	require.Equal(t, "output part", rl.Output())
	// ANSI attrs should come from the display portion only
	require.NotNil(t, rl.ANSIAttrs())
}

func TestNewRaw_ANSIEnabled_WithSepNoANSI(t *testing.T) {
	rl := NewRaw(1, "display\x00output", true, true)
	require.Equal(t, "display", rl.DisplayString())
	require.Nil(t, rl.ANSIAttrs())
	require.Equal(t, "output", rl.Output())
}
