package filter

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// NewFuzzy builds a fuzzy-finder type of filter.
// In effect, this uses a smart case filter, and for q query
// like "ABC" it matches the equivalent of "A(.*)B(.*)C(.*)"
func NewFuzzy() *Fuzzy {
	return &Fuzzy{}
}

func (ff Fuzzy) BufSize() int {
	return 0
}

func (ff *Fuzzy) NewContext(ctx context.Context, query string) context.Context {
	return newContext(ctx, query)
}

func (ff Fuzzy) String() string {
	return "Fuzzy"
}

func (ff *Fuzzy) Apply(ctx context.Context, lines []line.Line, out pipeline.ChanOutput) error {
	originalQuery := ctx.Value(queryKey).(string)
	hasUpper := util.ContainsUpper(originalQuery)

OUTER:
	for _, l := range lines {
		base := 0
		matches := [][]int{}
		txt := l.DisplayString()
		query := originalQuery
		for len(query) > 0 {
			r, n := utf8.DecodeRuneInString(query)
			query = query[n:]
			if r == utf8.RuneError {
				// "Silently" ignore
				continue OUTER
			}

			var i int
			if hasUpper { // explicit match
				i = strings.IndexRune(txt, r)
			} else {
				i = strings.IndexFunc(txt, util.CaseInsensitiveIndexFunc(r))
			}
			if i == -1 {
				continue OUTER
			}

			// otherwise we have a match, but the next match must match against
			// something AFTER the current match
			txt = txt[i+n:]
			matches = append(matches, []int{base + i, base + i + n})
			base = base + i + n
		}
		_ = out.Send(line.NewMatched(l, matches))
	}
	return nil
}
