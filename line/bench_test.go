package line

import (
	"fmt"
	"testing"
)

// BenchmarkMatchedLifecycle benchmarks the create-use-release cycle for
// Matched objects, simulating multiple keystroke filter runs. On the first
// iteration all objects are freshly allocated; on subsequent iterations
// they come from the pool.
func BenchmarkMatchedLifecycle(b *testing.B) {
	const numLines = 10_000
	raws := make([]*Raw, numLines)
	for i := range raws {
		raws[i] = NewRaw(uint64(i), fmt.Sprintf("line %d content here", i), false, false)
	}
	indices := [][]int{{0, 4}, {5, 10}}

	b.Run("pooled_GetMatched", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			// Simulate a filter run: create 10k Matched objects
			matched := make([]*Matched, numLines)
			for i, r := range raws {
				matched[i] = GetMatched(r, indices)
			}
			// Simulate Reset(): release all back to pool
			for _, m := range matched {
				ReleaseMatched(m)
			}
		}
	})

	b.Run("unpooled_NewMatched", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			// Simulate a filter run: create 10k Matched objects (no pooling)
			matched := make([]*Matched, numLines)
			for i, r := range raws {
				matched[i] = NewMatched(r, indices)
			}
			// No release -- objects become garbage
			_ = matched
		}
	})
}
