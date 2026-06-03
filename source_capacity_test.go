package peco

import (
	"fmt"
	"strings"
	"testing"

	"github.com/peco/peco/line"
	"github.com/stretchr/testify/require"
)

// TestSourceCapacity verifies that a capacity-bounded Source keeps exactly the
// most recent `capacity` lines as new lines stream in, across many compaction
// boundaries, and that the backing storage stays bounded (amortized O(1)
// trimming rather than a full reallocation per append).
func TestSourceCapacity(t *testing.T) {
	const capacity = 4
	ig := newIDGen()
	go ig.Run(t.Context())

	s := NewSource("-", strings.NewReader(""), false, ig, capacity, false, false)

	const total = 25 // crosses several capacity-sized compaction windows
	for i := range total {
		s.Append(line.NewRaw(uint64(i), fmt.Sprintf("line%d", i), false, false))

		want := min(i+1, capacity)
		require.Equal(t, want, s.Size(), "Size must stay pinned at capacity once saturated")

		// The live window must be exactly the most-recently-appended lines.
		oldest := (i + 1) - s.Size()
		for j := range s.Size() {
			l, err := s.LineAt(j)
			require.NoError(t, err, "LineAt(%d) at append %d", j, i)
			require.Equal(t, fmt.Sprintf("line%d", oldest+j), l.DisplayString())
		}

		// Backing storage must not grow without bound.
		require.LessOrEqual(t, len(s.lines), 2*capacity,
			"backing window should stay bounded at ~2*capacity")
	}

	// linesInRange over the live window still returns a correct contiguous slice
	// after many compactions.
	rng := s.linesInRange(0, capacity)
	require.Len(t, rng, capacity)
	require.Equal(t, fmt.Sprintf("line%d", total-capacity), rng[0].DisplayString())
	require.Equal(t, fmt.Sprintf("line%d", total-1), rng[capacity-1].DisplayString())

	// Out-of-range access is rejected.
	_, err := s.LineAt(capacity)
	require.Error(t, err)
}

// TestSourceUnlimitedRetainsAll verifies that the default (capacity 0) keeps
// every appended line — the start-offset machinery must not kick in.
func TestSourceUnlimitedRetainsAll(t *testing.T) {
	ig := newIDGen()
	go ig.Run(t.Context())

	s := NewSource("-", strings.NewReader(""), false, ig, 0, false, false)

	const total = 50
	for i := range total {
		s.Append(line.NewRaw(uint64(i), fmt.Sprintf("line%d", i), false, false))
	}

	require.Equal(t, total, s.Size())
	require.Equal(t, 0, s.start, "start must stay 0 when capacity is unlimited")
	first, err := s.LineAt(0)
	require.NoError(t, err)
	require.Equal(t, "line0", first.DisplayString())
	last, err := s.LineAt(total - 1)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("line%d", total-1), last.DisplayString())
}
