package filter

import (
	"context"
	"fmt"
	"testing"

	"github.com/peco/peco/line"
	"github.com/stretchr/testify/assert"
)

type indexer interface {
	Indices() [][]int
}

// TestFuzzy tests a fuzzy filter against various inputs
func TestFuzzy(t *testing.T) {
	ctx := context.Background()

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
	filter := NewFuzzy()
	for i, v := range testValues {
		t.Run(fmt.Sprintf(`"%s" against "%s", expect "%t"`, v.input, v.query, v.selected), func(t *testing.T) {
			ctx = NewContext(ctx, v.query)
			l := line.NewRaw(uint64(i), v.input, false)
			res, err := filter.Apply(ctx, l)

			if !v.selected {
				if !assert.Error(t, err, "filter should fail") {
					return
				}
				if !assert.Nil(t, res, "return value should be nil") {
					return
				}
				return
			}

			if !assert.NoError(t, err, "filtering failed") {
				return
			}

			if !assert.NotNil(t, res, "return value should NOT be nil") {
				return
			}

			if !assert.Implements(t, (*indexer)(nil), res, "can call Indices()") {
				return
			}
			t.Logf("%#v", res.(indexer).Indices())
		})
	}
}
