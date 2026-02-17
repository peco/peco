package filter

import (
	"context"
	"testing"
	"time"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/stretchr/testify/require"
)

// TestApplyAndApplyCollectConsistency verifies that Apply (channel-based) and
// ApplyCollect (slice-based) produce identical results for every built-in
// filter type. This is the key invariant that the baseFilter extraction must
// preserve.
func TestApplyAndApplyCollectConsistency(t *testing.T) {
	lines := makeLines(
		"hello world",
		"hello tests",
		"goodbye world",
		"goodbye tests",
		"fuzzy matching example",
		"another line",
	)

	tests := []struct {
		name   string
		filter Filter
		query  string
	}{
		{"IgnoreCase", NewIgnoreCase(), "hello"},
		{"CaseSensitive", NewCaseSensitive(), "hello"},
		{"SmartCase", NewSmartCase(), "hello"},
		{"Regexp", NewRegexp(), "hello.*world"},
		{"IRegexp", NewIRegexp(), "HELLO"},
		{"Fuzzy", NewFuzzy(false), "hw"},
		{"FuzzyLongest", NewFuzzy(true), "hw"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				tt.filter.NewContext(context.Background(), tt.query),
				10*time.Second,
			)
			defer cancel()

			// Collect via Apply (channel path)
			ch := make(chan line.Line, len(lines)+1)
			err := tt.filter.Apply(ctx, lines, pipeline.ChanOutput(ch))
			require.NoError(t, err, "Apply should succeed")
			close(ch)

			var applyResults []string
			for l := range ch {
				applyResults = append(applyResults, l.DisplayString())
			}

			// Collect via ApplyCollect (direct slice path)
			collector, ok := tt.filter.(Collector)
			require.True(t, ok, "%s should implement Collector", tt.name)

			ctx2, cancel2 := context.WithTimeout(
				tt.filter.NewContext(context.Background(), tt.query),
				10*time.Second,
			)
			defer cancel2()

			collected, err := collector.ApplyCollect(ctx2, lines)
			require.NoError(t, err, "ApplyCollect should succeed")

			collectResults := make([]string, 0, len(collected))
			for _, l := range collected {
				collectResults = append(collectResults, l.DisplayString())
			}

			// They must produce exactly the same results
			require.Equal(t, applyResults, collectResults,
				"Apply and ApplyCollect should produce identical results")
			require.NotEmpty(t, applyResults,
				"test should produce at least one match (verify test data)")
		})
	}
}

// TestBufSizeDefaults verifies that built-in filters return the expected
// BufSize (0 for Regexp/Fuzzy, meaning "use config default").
func TestBufSizeDefaults(t *testing.T) {
	tests := []struct {
		name     string
		filter   Filter
		expected int
	}{
		{"IgnoreCase", NewIgnoreCase(), 0},
		{"CaseSensitive", NewCaseSensitive(), 0},
		{"SmartCase", NewSmartCase(), 0},
		{"Regexp", NewRegexp(), 0},
		{"IRegexp", NewIRegexp(), 0},
		{"Fuzzy", NewFuzzy(false), 0},
		{"FuzzyLongest", NewFuzzy(true), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.filter.BufSize())
		})
	}
}

// TestNewContextStoresQuery verifies that NewContext stores the query and it
// can be retrieved by the filter's applyInternal.
func TestNewContextStoresQuery(t *testing.T) {
	filters := []struct {
		name   string
		filter Filter
	}{
		{"IgnoreCase", NewIgnoreCase()},
		{"Fuzzy", NewFuzzy(false)},
	}

	for _, tt := range filters {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.filter.NewContext(context.Background(), "test-query")
			// The query should be stored in context — verify by running Apply
			// with a line that matches "test-query"
			ch := make(chan line.Line, 2)
			lines := makeLines("this is a test-query line")
			err := tt.filter.Apply(ctx, lines, pipeline.ChanOutput(ch))
			require.NoError(t, err)
			close(ch)

			var results []line.Line
			for l := range ch {
				results = append(results, l)
			}
			require.Len(t, results, 1, "query stored by NewContext should be used by Apply")
		})
	}
}
