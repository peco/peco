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
