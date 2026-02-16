package peco

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseHeightSpec(t *testing.T) {
	t.Run("valid absolute", func(t *testing.T) {
		spec, err := ParseHeightSpec("10")
		require.NoError(t, err)
		require.Equal(t, 10, spec.Value)
		require.False(t, spec.IsPercent)
	})

	t.Run("valid percentage", func(t *testing.T) {
		spec, err := ParseHeightSpec("50%")
		require.NoError(t, err)
		require.Equal(t, 50, spec.Value)
		require.True(t, spec.IsPercent)
	})

	t.Run("valid 100%", func(t *testing.T) {
		spec, err := ParseHeightSpec("100%")
		require.NoError(t, err)
		require.Equal(t, 100, spec.Value)
		require.True(t, spec.IsPercent)
	})

	t.Run("valid with whitespace", func(t *testing.T) {
		spec, err := ParseHeightSpec("  10  ")
		require.NoError(t, err)
		require.Equal(t, 10, spec.Value)
		require.False(t, spec.IsPercent)
	})

	t.Run("invalid empty", func(t *testing.T) {
		_, err := ParseHeightSpec("")
		require.Error(t, err)
	})

	t.Run("invalid whitespace only", func(t *testing.T) {
		_, err := ParseHeightSpec("   ")
		require.Error(t, err)
	})

	t.Run("invalid abc", func(t *testing.T) {
		_, err := ParseHeightSpec("abc")
		require.Error(t, err)
	})

	t.Run("invalid negative", func(t *testing.T) {
		_, err := ParseHeightSpec("-5")
		require.Error(t, err)
	})

	t.Run("invalid zero", func(t *testing.T) {
		_, err := ParseHeightSpec("0")
		require.Error(t, err)
	})

	t.Run("invalid zero percent", func(t *testing.T) {
		_, err := ParseHeightSpec("0%")
		require.Error(t, err)
	})

	t.Run("invalid double percent", func(t *testing.T) {
		_, err := ParseHeightSpec("50%%")
		require.Error(t, err)
	})

	t.Run("invalid percent over 100", func(t *testing.T) {
		_, err := ParseHeightSpec("150%")
		require.Error(t, err)
	})
}

func TestHeightSpecResolve(t *testing.T) {
	// For absolute values: Value is result lines, total = Value + chromLines(2)
	t.Run("absolute adds chrome", func(t *testing.T) {
		spec := HeightSpec{Value: 10, IsPercent: false}
		// 10 result lines + 2 chrome = 12 total
		require.Equal(t, 12, spec.Resolve(24))
	})

	t.Run("absolute 1 result line", func(t *testing.T) {
		spec := HeightSpec{Value: 1, IsPercent: false}
		// 1 result line + 2 chrome = 3 total
		require.Equal(t, 3, spec.Resolve(24))
	})

	t.Run("absolute 2 result lines", func(t *testing.T) {
		spec := HeightSpec{Value: 2, IsPercent: false}
		// 2 result lines + 2 chrome = 4 total
		require.Equal(t, 4, spec.Resolve(24))
	})

	t.Run("absolute clamped to terminal height", func(t *testing.T) {
		spec := HeightSpec{Value: 50, IsPercent: false}
		// 50 + 2 = 52, clamped to 24
		require.Equal(t, 24, spec.Resolve(24))
	})

	t.Run("absolute clamped to minimum", func(t *testing.T) {
		// This shouldn't happen via ParseHeightSpec (rejects <= 0),
		// but Resolve handles it defensively.
		spec := HeightSpec{Value: 0, IsPercent: false}
		// 0 + 2 = 2, clamped to min 3
		require.Equal(t, chromLines+1, spec.Resolve(24))
	})

	// For percentages: Value is percentage of total terminal height
	t.Run("percentage", func(t *testing.T) {
		spec := HeightSpec{Value: 50, IsPercent: true}
		require.Equal(t, 12, spec.Resolve(24))
	})

	t.Run("percentage 100%", func(t *testing.T) {
		spec := HeightSpec{Value: 100, IsPercent: true}
		require.Equal(t, 24, spec.Resolve(24))
	})

	t.Run("percentage clamp to min", func(t *testing.T) {
		spec := HeightSpec{Value: 1, IsPercent: true}
		// 1% of 24 = 0, clamped to chromLines+1 = 3
		require.Equal(t, chromLines+1, spec.Resolve(24))
	})

	t.Run("percentage clamp to terminal height", func(t *testing.T) {
		spec := HeightSpec{Value: 100, IsPercent: true}
		require.Equal(t, 5, spec.Resolve(5))
	})

	t.Run("small terminal clamps absolute", func(t *testing.T) {
		spec := HeightSpec{Value: 10, IsPercent: false}
		// 10 + 2 = 12, clamped to 3
		require.Equal(t, 3, spec.Resolve(3))
	})
}
