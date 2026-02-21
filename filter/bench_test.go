package filter

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// BenchmarkFuzzyFilter benchmarks the fuzzy filter to measure allocations
// in the hot path (CaseInsensitiveIndexFunc closure, match slices, etc).
func BenchmarkFuzzyFilter(b *testing.B) {
	lines := make([]line.Line, 200)
	for i := range lines {
		lines[i] = line.NewRaw(uint64(i), "this is a reasonably long line for benchmarking the fuzzy filter path", false, false)
	}

	f := NewFuzzy(false)
	query := "trfp" // matches scattered chars across the line

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		ctx, cancel := context.WithTimeout(f.NewContext(context.Background(), query), 10*time.Second)
		ch := make(chan line.Line, len(lines))
		_ = f.Apply(ctx, lines, pipeline.ChanOutput(ch))
		cancel()
	}
}

// BenchmarkCaseInsensitiveIndexClosure measures the old closure-based approach.
func BenchmarkCaseInsensitiveIndexClosure(b *testing.B) {
	txt := "this is a reasonably long line for benchmarking"
	r := 'r'

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		fn := util.CaseInsensitiveIndexFunc(r)
		strings.IndexFunc(txt, fn)
	}
}

// BenchmarkCaseInsensitiveIndexDirect measures the new direct approach.
func BenchmarkCaseInsensitiveIndexDirect(b *testing.B) {
	txt := "this is a reasonably long line for benchmarking"
	r := 'r'

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		util.CaseInsensitiveIndex(txt, r)
	}
}

// BenchmarkRegexpFilter benchmarks the regexp filter with overlapping matches
// to exercise the dedup/merge path and matches slice allocation.
func BenchmarkRegexpFilter(b *testing.B) {
	lines := make([]line.Line, 200)
	for i := range lines {
		lines[i] = line.NewRaw(uint64(i), "the quick brown fox jumps over the lazy brown dog", false, false)
	}

	f := NewIgnoreCase()
	query := "brown the"

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		ctx, cancel := context.WithTimeout(f.NewContext(context.Background(), query), 10*time.Second)
		ch := make(chan line.Line, len(lines))
		_ = f.Apply(ctx, lines, pipeline.ChanOutput(ch))
		cancel()
	}
}

// BenchmarkRegexpFilterMultiCycle simulates multiple keystroke filter cycles.
// On each cycle, the filter produces Matched objects which are collected in a
// MemoryBuffer, then Reset() is called to release them before the next cycle.
// This demonstrates pool reuse across keystrokes.
func BenchmarkRegexpFilterMultiCycle(b *testing.B) {
	const numLines = 10_000
	lines := make([]line.Line, numLines)
	for i := range lines {
		lines[i] = line.NewRaw(uint64(i), "the quick brown fox jumps over the lazy dog", false, false)
	}

	f := NewIgnoreCase()
	query := "fox"

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// Simulate 3 keystroke cycles
		for cycle := 0; cycle < 3; cycle++ {
			ctx := f.NewContext(context.Background(), query)
			ch := make(chan line.Line, numLines)
			_ = f.Apply(ctx, lines, pipeline.ChanOutput(ch))
			close(ch)

			// Drain and release (simulating MemoryBuffer.Reset)
			for l := range ch {
				if m, ok := l.(*line.Matched); ok {
					line.ReleaseMatched(m)
				}
			}
		}
	}
}
