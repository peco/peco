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

// TestFuzzy tests a fuzzy filter against various inputs
func TestFuzzy(t *testing.T) {
	octx, ocancel := context.WithCancel(context.Background())
	defer ocancel()

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
