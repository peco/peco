package filter

import (
	"context"
	"fmt"
	"math"
	"regexp"
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
	ff := &Fuzzy{
		sortLongest: sortLongest,
	}
	ff.applyFn = ff.applyInternal
	return ff
}

func (ff Fuzzy) SupportsParallel() bool {
	return !ff.sortLongest
}

func (ff Fuzzy) String() string {
	return "Fuzzy"
}

func (ff *Fuzzy) applyInternal(ctx context.Context, lines []line.Line, emit func(line.Line)) error {
	originalQuery := pipeline.QueryFromContext(ctx)

	// Parse negative terms and compile them as case-insensitive regexps
	posTerms, negTerms := SplitQueryTerms(originalQuery)
	var negRegexps []*regexp.Regexp
	for _, t := range negTerms {
		re, err := regexpFor(t, []string{"i"}, true)
		if err != nil {
			return fmt.Errorf("failed to compile negative term regexp '%s': %w", t, err)
		}
		negRegexps = append(negRegexps, re)
	}

	// Reconstruct the fuzzy query from positive terms joined together
	fuzzyQuery := strings.Join(posTerms, "")

	hasUpper := util.ContainsUpper(fuzzyQuery)
	var matched []fuzzyMatchedItem

LINE:
	for i, l := range lines {
		if err := checkCancelled(ctx, i); err != nil {
			return err
		}

		txt := l.DisplayString()

		// Check negative terms first — skip if any match
		if isExcluded(negRegexps, txt) {
			continue LINE
		}

		// All-negative query: emit all non-excluded lines with nil indices
		if len(fuzzyQuery) == 0 {
			emit(line.NewMatched(l, nil))
			continue LINE
		}

		// Find the first valid rune of the query
		firstRune := utf8.RuneError
		for _, r := range fuzzyQuery {
			if r != utf8.RuneError {
				firstRune = r
				break
			}
		}
		if firstRune == utf8.RuneError {
			return fmt.Errorf("the query has no valid character")
		}

		// Find the index of the first valid rune in the input line
		txt = l.DisplayString()
		var firstRuneOffsets []int
		accum := 0
		var r rune
		var n int
		for len(txt) > 0 {
			txt, r, n = popRune(txt)
			var found bool
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
		var candidates []fuzzyMatchedItem

	OUTER:
		for _, offset := range firstRuneOffsets {
			query := fuzzyQuery
			txt = l.DisplayString()[offset:]
			base := offset
			var matches [][]int

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
		emit(line.NewMatched(matched[i].line, matched[i].matches))
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
		}
		if s[i].earliest != s[j].earliest {
			// Earlier match is better
			return s[i].earliest < s[j].earliest
		}
		// Shorter line is better
		return s[i].Len() < s[j].Len()
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
