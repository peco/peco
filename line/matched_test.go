package line

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAndReleaseMatched(t *testing.T) {
	t.Parallel()
	raw := NewRaw(1, "hello", false, false)
	indices := [][]int{{0, 5}}

	m := GetMatched(raw, indices)
	require.Equal(t, raw, m.Line)
	require.Equal(t, indices, m.Indices())
	require.Equal(t, "hello", m.DisplayString())

	ReleaseMatched(m)
	require.Nil(t, m.Line)
	require.Nil(t, m.indices)

	// Get again — should reuse the pooled object
	m2 := GetMatched(raw, nil)
	require.Equal(t, raw, m2.Line)
	require.Nil(t, m2.Indices())
	ReleaseMatched(m2)
}
