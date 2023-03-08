package filter

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// NewFuzzy builds a fuzzy-finder type of filter.
// In effect, this uses a smart case filter, and for q query
// like "ABC" it matches the equivalent of "A(.*)B(.*)C(.*)".
//
// With sortLongest = true, Fuzzy filter outputs the result
// sorted in the following precedence:
//  1. Longer match
//  2. Earlier match
//  3. Shorter line length
func NewFuzzy(sortLongest bool) *Fuzzy {
	return &Fuzzy{
		sortLongest: sortLongest,
	}
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
	matched := []fuzzyMatchedItem{}

LINE:
	for _, l := range lines {
		// Find the first valid rune of the query
		firstRune := utf8.RuneError
		for _, r := range originalQuery {
			if r != utf8.RuneError {
				firstRune = r
				break
			}
		}
		if firstRune == utf8.RuneError {
			return fmt.Errorf("the query has no valid character")
		}

		// Find the index of the first valid rune in the input line
		txt := l.DisplayString()
		firstRuneOffsets := []int{}
		accum := 0
		r := rune(0)
		n := 0
		for len(txt) > 0 {
			txt, r, n = popRune(txt)
			found := false
			if hasUpper {
				found = r == firstRune
			} else {
				found = unicode.ToUpper(r) == unicode.ToUpper(firstRune)
			}
			if found {
				firstRuneOffsets = append(firstRuneOffsets, accum)

				if !ff.sortLongest {
					// Old behavior only sees the first match
					break
				}
			}
			accum += n
		}
		if len(firstRuneOffsets) == 0 {
			continue LINE
		}

		// Find all candidate matches
		candidates := []fuzzyMatchedItem{}

	OUTER:
		for _, offset := range firstRuneOffsets {
			query := originalQuery
			txt = l.DisplayString()[offset:]
			base := offset
			matches := [][]int{}

			for len(query) > 0 {
				query, r, n = popRune(query)
				if r == utf8.RuneError {
					// "Silently" ignore
					continue OUTER
				}

				var i int
				if hasUpper {
					i = strings.IndexRune(txt, r)
				} else {
					i = strings.IndexFunc(txt, util.CaseInsensitiveIndexFunc(r))
				}
				if i == -1 {
					continue OUTER
				}

				txt = txt[i+n:]
				matches = append(matches, []int{base + i, base + i + n})
				base = base + i + n
			}

			candidates = append(candidates, newFuzzyMatchedItem(l, matches))
		}

		if len(candidates) == 0 {
			continue
		}

		if ff.sortLongest {
			// Sort the candidate matches of a line and pick the best one
			sort.SliceStable(candidates, less(candidates))
		}
		matched = append(matched, candidates[0])
	}

	if ff.sortLongest {
		// Sort all matched lines
		sort.SliceStable(matched, less(matched))
	}

	for i := range matched {
		out.Send(line.NewMatched(matched[i].line, matched[i].matches))
	}

	return nil
}

func popRune(s string) (string, rune, int) {
	r, n := utf8.DecodeRuneInString(s)
	return s[n:], r, n
}

func less(s []fuzzyMatchedItem) func(i, j int) bool {
	return func(i, j int) bool {
		if s[i].longest != s[j].longest {
			// Longer match is better
			return s[i].longest > s[j].longest
		} else if s[i].earliest != s[j].earliest {
			// Earlier match is better
			return s[i].earliest < s[j].earliest
		} else {
			// Shorter line is better
			return s[i].Len() < s[j].Len()
		}
	}
}

type fuzzyMatchedItem struct {
	line     line.Line
	matches  [][]int
	longest  int
	earliest int
}

func newFuzzyMatchedItem(line line.Line, matches [][]int) fuzzyMatchedItem {
	longest := 0
	count := 0
	lastEnd := 0
	earliest := math.MaxInt

	for i := range matches {
		length := matches[i][1] - matches[i][0]
		if matches[i][0] == lastEnd {
			count += length
		} else {
			count = length
		}
		if count > longest {
			longest = count
		}
		lastEnd = matches[i][1]

		if matches[i][0] < earliest {
			earliest = matches[i][0]
		}
	}

	return fuzzyMatchedItem{
		line:     line,
		matches:  matches,
		longest:  longest,
		earliest: earliest,
	}
}

func (f fuzzyMatchedItem) Len() int {
	return len(f.line.DisplayString())
}
