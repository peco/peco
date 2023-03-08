package filter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/stretchr/testify/assert"
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
	octx, ocancel := context.WithCancel(context.Background())
	defer ocancel()

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

			ch := make(chan interface{}, 1)
			l := line.NewRaw(uint64(i), v.input, false)
			err := filter.Apply(ctx, []line.Line{l}, pipeline.ChanOutput(ch))
			if !assert.NoError(t, err, `filter.Apply should succeed`) {
				return
			}

			select {
			case l, ok := <-ch:
				if !assert.True(t, ok, `channel read should succeed`) {
					return
				}

				if !assert.Implements(t, (*line.Line)(nil), l, "result is a line") {
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
				lines = append(lines, line.NewRaw(uint64(i), raw, false))
			}

			var actual []string
			lc := make(chan interface{})
			ec := make(chan error)
			go func() {
				ec <- filter.Apply(ctx, lines, lc)
			}()

		OUTER:
			for {
				select {
				case l := <-lc:
					if !assert.Implements(t, (*line.Line)(nil), l, "result is a line") {
						return
					}
					actual = append(actual, l.(line.Line).DisplayString())
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
			lc := make(chan interface{})
			ec := make(chan error)
			go func() {
				ec <- filter.Apply(ctx, []line.Line{line.NewRaw(uint64(i), v.input, false)}, lc)
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
