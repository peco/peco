package filter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type indexer interface {
	Indices() [][]int
}

// TestFuzzy tests the Fuzzy filter against various inputs.
//
//	testFuzzy: simple substring match
//	testFuzzyLongest: sorted longest substring match
//	testFuzzyMatch: match position without sorting
//	testFuzzyLongestMatch: match position with sorting
func TestFuzzy(t *testing.T) {
	octx := t.Context()

	testFuzzy(octx, t, NewFuzzy(false))
	testFuzzyLongest(octx, t, NewFuzzy(true))
	testFuzzyMatch(octx, t, NewFuzzy(false))
}

// testFuzzy tests if given filter matches/rejects the query.
// This test checks the following functionalities:
//   - Fuzzy substring match
//   - Case-insensitive match
//   - Multi-byte rune match
func testFuzzy(octx context.Context, t *testing.T, filter Filter) {
	testValues := []struct {
		input    string
		query    string
		selected bool
	}{
		{"this is a test to test the fuzzy Filter", "tf", true},  // normal selection
		{"this is a test to test the fuzzy Filter", "wp", false}, // incorrect selection
		{"THIS IS A TEST TO TEST THE FUZZY FILTER", "tu", true},  // case insensitivity
		{"this is a Test to test the fuzzy filter", "Tu", true},  // case sensitivity
		{"this is a Test to test the fUzzy filter", "TU", true},  // case sensitivity
		{"this is a test to test the fuzzy filter", "Tu", false}, // case sensitivity
		{"this is a test to Test the fuzzy filter", "TU", false}, // case sensitivity
		{"日本語は難しいです", "難", true},                                 // kanji
		{"あ、日本語は難しいですよ", "あい", true},                             // hiragana
		{"パソコンは遅いですネ", "ソネ", true},                               // katakana
		{"🚴🏻 abcd efgh", "🚴🏻e", true},                            // unicode
		{"This is a test to Test the fuzzy filteR", "TTR", true},
	}
	for i, v := range testValues {
		t.Run(fmt.Sprintf(`"%s" against "%s", expect "%t"`, v.input, v.query, v.selected), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(filter.NewContext(octx, v.query), 10*time.Second)
			defer cancel()

			ch := make(chan line.Line, 1)
			l := line.NewRaw(uint64(i), v.input, false, false)
			err := filter.Apply(ctx, []line.Line{l}, pipeline.ChanOutput(ch))
			if !assert.NoError(t, err, `filter.Apply should succeed`) {
				return
			}

			select {
			case l, ok := <-ch:
				if !assert.True(t, ok, `channel read should succeed`) {
					return
				}

				t.Logf("%#v", l.(indexer).Indices())
			case <-ctx.Done():
				if !assert.False(t, v.selected, "did NOT expect to timeout") { // shouldn't happen if we're expecting a result
					return
				}
			}
		})
	}
}

// testFuzzyLongest tests if given filter matches/rejects the query.
// This test check the following functionalities:
//   - Longest substring match
//   - Ordering the match result by precedence (substring length > match index > string length)
func testFuzzyLongest(octx context.Context, t *testing.T, filter Filter) {
	testValues := []struct {
		name   string
		query  string
		input  []string
		expect []string
	}{
		{
			name:  "The longer the matched string, the higher it ranks",
			query: "abcd",
			input: []string{
				"abc-d",
				"ab-cd",
				"abcd",
				"a-bcd",
			},
			expect: []string{
				"abcd",
				"abc-d",
				"a-bcd",
				"ab-cd",
			},
		},
		{
			name:  "The earlier it matches, the higher it ranks",
			query: "abcd",
			input: []string{
				"___abcd",
				"_abcd",
				"abcd",
				"__abcd",
			},
			expect: []string{
				"abcd",
				"_abcd",
				"__abcd",
				"___abcd",
			},
		},
		{
			name:  "The shorter the original string, the higher it ranks",
			query: "abcd",
			input: []string{
				"abcdef",
				"abcdefg",
				"abcd",
				"abcde",
			},
			expect: []string{
				"abcd",
				"abcde",
				"abcdef",
				"abcdefg",
			},
		},
		{
			name:  "Mixed precedence",
			query: "abcd",
			input: []string{
				"abc-d",
				"ab-cd",
				"abcd",
				"ab_abcd",
				"a-bcd",
				"___abcd",
				"_abcd",
				"abcd",
				"__abcd",
				"abcdef",
				"abcdefg",
				"abcd",
				"abcde",
			},
			expect: []string{
				"abcd",
				"abcd",
				"abcd",
				"abcde",
				"abcdef",
				"abcdefg",
				"_abcd",
				"__abcd",
				"ab_abcd", // ab_abcd shall be above ___abcd because matched lines are stable-sorted
				"___abcd",
				"abc-d",
				"a-bcd",
				"ab-cd",
			},
		},
	}

	for i, v := range testValues {
		t.Run(v.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(filter.NewContext(octx, v.query), 10*time.Second)
			defer cancel()

			var lines []line.Line
			for _, raw := range v.input {
				lines = append(lines, line.NewRaw(uint64(i), raw, false, false))
			}

			var actual []string
			lc := make(chan line.Line)
			ec := make(chan error)
			go func() {
				ec <- filter.Apply(ctx, lines, pipeline.ChanOutput(lc))
			}()

		OUTER:
			for {
				select {
				case l := <-lc:
					actual = append(actual, l.DisplayString())
				case err := <-ec:
					if !assert.NoError(t, err, `filter.Apply should succeed`) {
						return
					}
					break OUTER
				case <-ctx.Done():
					t.Fatalf("unexpected timeout")
				}
			}

			if !assert.Equal(t, v.expect, actual, "result is ordered in expected order") {
				return
			}
		})
	}
}

// testFuzzyMatch tests if non-sorted & sorted Fuzzy filter returns the expected result
func TestSplitQueryTerms(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantPos []string
		wantNeg []string
	}{
		{
			name:    "simple positive",
			query:   "foo bar",
			wantPos: []string{"foo", "bar"},
		},
		{
			name:    "single negative",
			query:   "-foo",
			wantNeg: []string{"foo"},
		},
		{
			name:    "mixed positive and negative",
			query:   "foo -bar baz",
			wantPos: []string{"foo", "baz"},
			wantNeg: []string{"bar"},
		},
		{
			name:    "all negative",
			query:   "-foo -bar",
			wantNeg: []string{"foo", "bar"},
		},
		{
			name:    "escaped negative becomes positive",
			query:   `\-foo`,
			wantPos: []string{"-foo"},
		},
		{
			name:    "bare hyphen is positive literal",
			query:   "-",
			wantPos: []string{"-"},
		},
		{
			name:    "double hyphen is positive literal",
			query:   "--",
			wantPos: []string{"--"},
		},
		{
			name:    "mixed with escaping",
			query:   `foo -bar \-baz`,
			wantPos: []string{"foo", "-baz"},
			wantNeg: []string{"bar"},
		},
		{
			name:    "extra spaces are skipped",
			query:   "  foo   -bar  ",
			wantPos: []string{"foo"},
			wantNeg: []string{"bar"},
		},
		{
			name:  "empty query",
			query: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPos, gotNeg := SplitQueryTerms(tt.query)
			require.Equal(t, tt.wantPos, gotPos, "positive terms")
			require.Equal(t, tt.wantNeg, gotNeg, "negative terms")
		})
	}
}

// collectFilterResults runs the filter and collects all emitted lines.
func collectFilterResults(t *testing.T, f Filter, query string, inputLines []line.Line) []line.Line {
	t.Helper()
	ctx, cancel := context.WithTimeout(f.NewContext(context.Background(), query), 10*time.Second)
	defer cancel()

	ch := make(chan line.Line, len(inputLines)+1)
	err := f.Apply(ctx, inputLines, pipeline.ChanOutput(ch))
	require.NoError(t, err, "filter.Apply should succeed")
	close(ch)

	var results []line.Line
	for l := range ch {
		results = append(results, l)
	}
	return results
}

func makeLines(inputs ...string) []line.Line {
	lines := make([]line.Line, len(inputs))
	for i, s := range inputs {
		lines[i] = line.NewRaw(uint64(i), s, false, false)
	}
	return lines
}

func TestNegativeMatchingRegexp(t *testing.T) {
	filters := map[string]Filter{
		"IgnoreCase":    NewIgnoreCase(),
		"CaseSensitive": NewCaseSensitive(),
		"SmartCase":     NewSmartCase(),
		"Regexp":        NewRegexp(),
	}

	for name, f := range filters {
		t.Run(name, func(t *testing.T) {
			lines := makeLines(
				"hello world",
				"hello tests",
				"goodbye world",
				"goodbye tests",
			)

			t.Run("positive with negative exclusion", func(t *testing.T) {
				results := collectFilterResults(t, f, "hello -tests", lines)
				require.Len(t, results, 1)
				require.Equal(t, "hello world", results[0].DisplayString())
			})

			t.Run("multiple negative terms", func(t *testing.T) {
				results := collectFilterResults(t, f, "hello -world -tests", lines)
				require.Len(t, results, 0)
			})
		})
	}
}

func TestAllNegativeQuery(t *testing.T) {
	filters := map[string]Filter{
		"IgnoreCase": NewIgnoreCase(),
		"Regexp":     NewRegexp(),
		"Fuzzy":      NewFuzzy(false),
	}

	lines := makeLines(
		"alpha",
		"beta",
		"gamma",
	)

	for name, f := range filters {
		t.Run(name, func(t *testing.T) {
			results := collectFilterResults(t, f, "-beta", lines)
			require.Len(t, results, 2)
			names := make([]string, 0, len(results))
			for _, r := range results {
				names = append(names, r.DisplayString())
			}
			require.Contains(t, names, "alpha")
			require.Contains(t, names, "gamma")
		})
	}
}

func TestNegativeNoHighlight(t *testing.T) {
	f := NewIgnoreCase()
	lines := makeLines("alpha", "beta", "gamma")
	results := collectFilterResults(t, f, "-beta", lines)
	require.Len(t, results, 2)

	for _, r := range results {
		idx, ok := r.(indexer)
		require.True(t, ok, "result should implement indexer")
		require.Nil(t, idx.Indices(), "all-negative query should produce nil indices")
	}
}

func TestNegativeMatchingFuzzy(t *testing.T) {
	f := NewFuzzy(false)
	lines := makeLines(
		"hello world",
		"hello tests",
		"goodbye world",
		"goodbye tests",
	)

	t.Run("fuzzy positive with negative exclusion", func(t *testing.T) {
		// Fuzzy query "hlo" should match "hello" lines; -tests excludes one
		results := collectFilterResults(t, f, "hlo -tests", lines)
		require.Len(t, results, 1)
		require.Equal(t, "hello world", results[0].DisplayString())
	})

	t.Run("fuzzy all-negative", func(t *testing.T) {
		results := collectFilterResults(t, f, "-world", lines)
		require.Len(t, results, 2)
		names := make([]string, 0, len(results))
		for _, r := range results {
			names = append(names, r.DisplayString())
		}
		require.Contains(t, names, "hello tests")
		require.Contains(t, names, "goodbye tests")
	})
}

func TestLiteralHyphenMatching(t *testing.T) {
	lines := makeLines(
		"hello-world",
		"hello world",
		"-foo bar",
		"foo bar",
		"--verbose flag",
		"verbose flag",
	)

	f := NewIgnoreCase()

	t.Run("escaped negative matches literal hyphen-prefixed term", func(t *testing.T) {
		// \-foo should match lines containing literal "-foo"
		results := collectFilterResults(t, f, `\-foo`, lines)
		require.Len(t, results, 1)
		require.Equal(t, "-foo bar", results[0].DisplayString())
	})

	t.Run("bare hyphen matches lines containing hyphen", func(t *testing.T) {
		// bare "-" should be a positive literal matching any line with a hyphen
		results := collectFilterResults(t, f, "-", lines)
		require.Len(t, results, 3)
		names := make([]string, 0, len(results))
		for _, r := range results {
			names = append(names, r.DisplayString())
		}
		require.Contains(t, names, "hello-world")
		require.Contains(t, names, "-foo bar")
		require.Contains(t, names, "--verbose flag")
	})

	t.Run("double hyphen matches lines containing double hyphen", func(t *testing.T) {
		results := collectFilterResults(t, f, "--", lines)
		require.Len(t, results, 1)
		require.Equal(t, "--verbose flag", results[0].DisplayString())
	})

	t.Run("escaped negative with positive term", func(t *testing.T) {
		// Search for lines containing both "bar" and literal "-foo"
		results := collectFilterResults(t, f, `bar \-foo`, lines)
		require.Len(t, results, 1)
		require.Equal(t, "-foo bar", results[0].DisplayString())
	})
}

// testFuzzyMatch tests if non-sorted & sorted Fuzzy filter returns the expected result
func testFuzzyMatch(octx context.Context, t *testing.T, filter Filter) {
	testValues := []struct {
		name   string
		sort   bool
		query  string
		input  string
		expect [][]int
	}{
		{
			name:  "Fuzzy: exact match",
			sort:  false,
			query: "asdf",
			input: "___asdf",
			//         ^^^^
			expect: [][]int{
				{3, 4},
				{4, 5},
				{5, 6},
				{6, 7},
			},
		},
		{
			name:  "Fuzzy: scattered match",
			sort:  false,
			query: "asdf",
			input: "as_asdf",
			//      ^^   ^^
			expect: [][]int{
				{0, 1},
				{1, 2},
				{5, 6},
				{6, 7},
			},
		},
		{
			name:  "FuzzyLongest: exact match",
			sort:  true,
			query: "asdf",
			input: "___asdf",
			//         ^^^^
			expect: [][]int{
				{3, 4},
				{4, 5},
				{5, 6},
				{6, 7},
			},
		},
		{
			name:  "FuzzyLongest: scattered match",
			sort:  true,
			query: "asdf",
			input: "as_asdf",
			//         ^^^^
			expect: [][]int{
				{3, 4},
				{4, 5},
				{5, 6},
				{6, 7},
			},
		},
	}

	for i, v := range testValues {
		t.Run(v.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(filter.NewContext(octx, v.query), 10*time.Second)
			defer cancel()

			filter := NewFuzzy(v.sort)
			lc := make(chan line.Line)
			ec := make(chan error)
			go func() {
				ec <- filter.Apply(ctx, []line.Line{line.NewRaw(uint64(i), v.input, false, false)}, pipeline.ChanOutput(lc))
			}()

		OUTER:
			for {
				select {
				case l := <-lc:
					if !assert.Implements(t, (*indexer)(nil), l, "result is an indexer") {
						return
					}
					if !assert.Equal(t, v.expect, l.(indexer).Indices(), "result has expected indices") {
						return
					}
				case err := <-ec:
					if !assert.NoError(t, err, `filter.Apply should succeed`) {
						return
					}
					break OUTER
				case <-ctx.Done():
					t.Fatalf("unexpected timeout")
				}
			}
		})
	}
}
