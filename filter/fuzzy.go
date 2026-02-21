package filter

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// Fuzzy is a filter that performs fuzzy matching.
type Fuzzy struct {
	baseFilter
	sortLongest bool
}

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
	ff.impl = ff
	return ff
}

func (ff Fuzzy) SupportsParallel() bool {
	return !ff.sortLongest
}

func (ff Fuzzy) String() string {
	return "Fuzzy"
}

// applyInternal performs fuzzy matching on each line, emitting matches with
// their character-level match indices for highlighting.
func (ff *Fuzzy) applyInternal(ctx context.Context, lines []line.Line, em LineEmitter) error {
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
			em.Emit(ctx, line.NewMatched(l, nil))
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

		// Find all byte offsets where the first query rune appears,
		// reusing txt from line 77 instead of re-fetching DisplayString().
		var firstRuneOffsets []int
		remaining := txt
		base := 0
		for len(remaining) > 0 {
			var idx int
			if hasUpper {
				idx = strings.IndexRune(remaining, firstRune)
			} else {
				idx = strings.IndexFunc(remaining, util.CaseInsensitiveIndexFunc(firstRune))
			}
			if idx == -1 {
				break
			}

			firstRuneOffsets = append(firstRuneOffsets, base+idx)

			if !ff.sortLongest {
				break
			}

			_, n := utf8.DecodeRuneInString(remaining[idx:])
			remaining = remaining[idx+n:]
			base += idx + n
		}
		if len(firstRuneOffsets) == 0 {
			continue LINE
		}

		// Find all candidate matches
		var candidates []fuzzyMatchedItem

		// Pre-allocate backing array for match pairs to avoid per-match
		// heap allocations. Each match is a [start, end] pair, so we need
		// 2 ints per query rune (upper bound).
		queryRuneCount := utf8.RuneCountInString(fuzzyQuery)
		matchBacking := make([]int, 0, queryRuneCount*2)

	OUTER:
		for _, offset := range firstRuneOffsets {
			q := fuzzyQuery
			candidateTxt := txt[offset:]
			candidateBase := offset
			matchBacking = matchBacking[:0]
			matches := make([][]int, 0, queryRuneCount)

			for len(q) > 0 {
				var r rune
				var n int
				q, r, n = popRune(q)
				if r == utf8.RuneError {
					continue OUTER
				}

				var idx int
				if hasUpper {
					idx = strings.IndexRune(candidateTxt, r)
				} else {
					idx = strings.IndexFunc(candidateTxt, util.CaseInsensitiveIndexFunc(r))
				}
				if idx == -1 {
					continue OUTER
				}

				candidateTxt = candidateTxt[idx+n:]
				start := candidateBase + idx
				matchBacking = append(matchBacking, start, start+n)
				matches = append(matches, matchBacking[len(matchBacking)-2:len(matchBacking):len(matchBacking)])
				candidateBase = start + n
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
		em.Emit(ctx, line.NewMatched(matched[i].line, matched[i].matches))
	}

	return nil
}

// popRune decodes and removes the first rune from s, returning the remainder,
// the decoded rune, and its byte width.
func popRune(s string) (string, rune, int) {
	r, n := utf8.DecodeRuneInString(s)
	return s[n:], r, n
}

// less returns a comparison function that orders fuzzy matches by longest
// contiguous match, earliest position, then shortest line length.
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

// newFuzzyMatchedItem creates a fuzzyMatchedItem, computing the longest
// contiguous match length and earliest match position from the given indices.
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
