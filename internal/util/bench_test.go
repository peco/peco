package util

import (
	"fmt"
	"testing"
)

func BenchmarkStripANSISequenceBulk(b *testing.B) {
	b.Run("10k_plain_lines", func(b *testing.B) {
		lines := make([]string, 10_000)
		for i := range lines {
			lines[i] = fmt.Sprintf("line %d: this is a typical line without any ansi codes at all padding", i)
		}
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			for _, l := range lines {
				StripANSISequence(l)
			}
		}
	})

	b.Run("10k_mixed_95pct_plain", func(b *testing.B) {
		lines := make([]string, 10_000)
		for i := range lines {
			if i%20 == 0 {
				lines[i] = fmt.Sprintf("\x1b[31mline %d\x1b[0m: this has ansi codes in it for color", i)
			} else {
				lines[i] = fmt.Sprintf("line %d: this is a typical line without any ansi codes at all padding", i)
			}
		}
		b.ResetTimer()
		b.ReportAllocs()
		for b.Loop() {
			for _, l := range lines {
				StripANSISequence(l)
			}
		}
	})
}
