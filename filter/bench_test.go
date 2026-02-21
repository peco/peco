package filter

import (
	"context"
	"testing"
	"time"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// BenchmarkFuzzyFilter benchmarks the fuzzy filter to measure allocations
// in the hot path.
func BenchmarkFuzzyFilter(b *testing.B) {
	lines := make([]line.Line, 200)
	for i := range lines {
		lines[i] = line.NewRaw(uint64(i), "this is a reasonably long line for benchmarking the fuzzy filter path", false, false)
	}

	f := NewFuzzy(false)
	query := "trfp"

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		ctx, cancel := context.WithTimeout(f.NewContext(context.Background(), query), 10*time.Second)
		ch := make(chan line.Line, len(lines))
		_ = f.Apply(ctx, lines, pipeline.ChanOutput(ch))
		cancel()
	}
}
