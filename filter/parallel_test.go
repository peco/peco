package filter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/stretchr/testify/require"
)

func TestSupportsParallel(t *testing.T) {
	tests := []struct {
		name     string
		filter   Filter
		expected bool
	}{
		{"Regexp", NewRegexp(), true},
		{"IgnoreCase", NewIgnoreCase(), true},
		{"CaseSensitive", NewCaseSensitive(), true},
		{"SmartCase", NewSmartCase(), true},
		{"IRegexp", NewIRegexp(), true},
		{"Fuzzy(sortLongest=false)", NewFuzzy(false), true},
		{"Fuzzy(sortLongest=true)", NewFuzzy(true), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.filter.SupportsParallel())
		})
	}
}

func TestParallelFilterProducesSameResults(t *testing.T) {
	// Generate test lines
	const numLines = 5000
	lines := make([]line.Line, numLines)
	for i := range numLines {
		text := fmt.Sprintf("line-%04d foo bar baz", i)
		if i%3 == 0 {
			text = fmt.Sprintf("line-%04d matching-pattern test", i)
		}
		lines[i] = line.NewRaw(uint64(i), text, false, false)
	}

	filters := []struct {
		name   string
		filter Filter
		query  string
	}{
		{"IgnoreCase", NewIgnoreCase(), "matching"},
		{"CaseSensitive", NewCaseSensitive(), "matching"},
		{"Regexp", NewRegexp(), "matching.*test"},
		{"Fuzzy", NewFuzzy(false), "mpt"},
	}

	for _, ft := range filters {
		t.Run(ft.name, func(t *testing.T) {
			ctx := ft.filter.NewContext(context.Background(), ft.query)

			// Run sequentially
			seqCh := make(chan line.Line, numLines)
			err := ft.filter.Apply(ctx, lines, pipeline.ChanOutput(seqCh))
			require.NoError(t, err)
			close(seqCh)

			var seqResults []string
			for l := range seqCh {
				seqResults = append(seqResults, l.DisplayString())
			}

			// Run on chunks (simulating parallel) - split into multiple chunks
			chunkSize := 500
			var parResults []string
			for start := 0; start < len(lines); start += chunkSize {
				end := min(start+chunkSize, len(lines))
				chunk := lines[start:end]

				ch := make(chan line.Line, len(chunk))
				err := ft.filter.Apply(ctx, chunk, pipeline.ChanOutput(ch))
				require.NoError(t, err)
				close(ch)

				for l := range ch {
					parResults = append(parResults, l.DisplayString())
				}
			}

			// Results should be identical in count and order
			require.Equal(t, len(seqResults), len(parResults), "result count should match")
			require.Equal(t, seqResults, parResults, "results should be identical and in same order")
		})
	}
}

func TestParallelFilterContextCancellation(t *testing.T) {
	// Generate a large set of lines
	const numLines = 100000
	lines := make([]line.Line, numLines)
	for i := range numLines {
		lines[i] = line.NewRaw(uint64(i), fmt.Sprintf("line-%d matching-pattern", i), false, false)
	}

	f := NewIgnoreCase()
	ctx, cancel := context.WithCancel(f.NewContext(context.Background(), "matching"))

	// Cancel after a short delay
	go func() {
		time.Sleep(1 * time.Millisecond)
		cancel()
	}()

	ch := make(chan line.Line, numLines)
	err := f.Apply(ctx, lines, pipeline.ChanOutput(ch))

	// Should return context error (cancelled)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}
