package peco

import (
	"strconv"
	"strings"
	"testing"

	"github.com/peco/peco/line"
)

// BenchmarkSourceAppendSaturated measures Append cost on a capacity-bounded
// source that is already saturated, which is the steady state for a long-lived
// streaming input (e.g. `tail -f | peco -b N`).
func BenchmarkSourceAppendSaturated(b *testing.B) {
	const capacity = 10000
	ig := newIDGen()
	go ig.Run(b.Context())

	s := NewSource("-", strings.NewReader(""), false, ig, capacity, false, false)
	// Pre-fill to capacity so every benchmarked Append discards an old line.
	for i := range capacity {
		s.Append(line.NewRaw(uint64(i), strconv.Itoa(i), false, false))
	}

	l := line.NewRaw(uint64(capacity), "payload", false, false)
	for b.Loop() {
		s.Append(l)
	}
}
