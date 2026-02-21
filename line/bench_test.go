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

// BenchmarkNewRawAndDisplay benchmarks creating Raw lines and calling
// DisplayString(), simulating peco's startup + render path for 10k lines.
func BenchmarkNewRawAndDisplay(b *testing.B) {
	texts := make([]string, 10_000)
	for i := range texts {
		texts[i] = fmt.Sprintf("line %d: this is a typical plain text line without any ansi codes", i)
	}

	b.Run("enableANSI_true_plain_text", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for i, t := range texts {
				r := NewRaw(uint64(i), t, false, true)
				_ = r.DisplayString()
			}
		}
	})

	b.Run("enableANSI_false_plain_text", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for i, t := range texts {
				r := NewRaw(uint64(i), t, false, false)
				_ = r.DisplayString()
			}
		}
	})
}
