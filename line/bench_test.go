package line

import (
	"fmt"
	"testing"
)

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
