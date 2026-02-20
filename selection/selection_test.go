package selection

import (
	"testing"

	"github.com/peco/peco/line"
	"github.com/stretchr/testify/require"
)

func TestSelection(t *testing.T) {
	s := New()

	var i uint64
	alice := line.NewRaw(i, "Alice", false, false)
	i++
	s.Add(alice)
	require.Equal(t, 1, s.Len())
	s.Add(line.NewRaw(i, "Bob", false, false))
	require.Equal(t, 2, s.Len())
	s.Add(alice)
	require.Equal(t, 2, s.Len())
	s.Remove(alice)
	require.Equal(t, 1, s.Len())
}
