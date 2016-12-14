package filter

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
)

// NewFuzzy builds a fuzzy-finder type of filter.
// In effect, this uses a smart case filter, and for q query
// like "ABC" it matches the equivalent of "A(.*)B(.*)C(.*)"
func NewFuzzy() *Fuzzy {
	return &Fuzzy{}
}

func (ff Fuzzy) String() string {
	return "Fuzzy"
}

func (ff *Fuzzy) Apply(ctx context.Context, l line.Line) (line.Line, error) {
	query := ctx.Value(queryKey).(string)
	base := 0
	txt := l.DisplayString()
	matches := [][]int{}

	hasUpper := util.ContainsUpper(query)

	for len(query) > 0 {
		r, n := utf8.DecodeRuneInString(query)
		if r == utf8.RuneError {
			// "Silently" ignore (just return a no match)
			return nil, errors.New("failed to decode input string")
		}
		query = query[n:]

		var i int
		if hasUpper { // explicit match
			i = strings.IndexRune(txt, r)
		} else {
			i = strings.IndexFunc(txt, util.CaseInsensitiveIndexFunc(r))
		}
		if i == -1 {
			return nil, errors.New("filter did not match against given line")
		}

		// otherwise we have a match, but the next match must match against
		// something AFTER the current match
		txt = txt[i+n:]
		matches = append(matches, []int{base + i, base + i + n})
		base = base + i + n
	}
	return line.NewMatched(l, matches), nil
}
