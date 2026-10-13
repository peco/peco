package filter

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// internal stuff
type regexpFlags interface {
	flags(string) []string
}

type regexpFlagList []string

type regexpFlagFunc func(string) []string

type regexpQueryFactory struct {
	compiled       map[string]regexpQuery
	mutex          sync.Mutex
	negationPrefix string
	threshold      time.Duration
}

type regexpQuery struct {
	positive []*regexp.Regexp
	negative []*regexp.Regexp
	lastUsed time.Time
}

var ignoreCaseFlags = regexpFlagList([]string{"i"})
var defaultFlags = regexpFlagList{}

// Regexp is a filter that matches lines using regular expressions.
type Regexp struct {
	baseFilter
	factory   *regexpQueryFactory
	flags     regexpFlags
	quotemeta bool
	name      string
}

func (r regexpFlagList) flags(_ string) []string {
	return []string(r)
}

func (r regexpFlagFunc) flags(s string) []string {
	return r(s)
}

// regexpFor compiles q into a regexp, optionally quoting meta characters
// and prepending inline flags (e.g. case-insensitive).
func regexpFor(q string, flags []string, quotemeta bool) (*regexp.Regexp, error) {
	reTxt := q
	if quotemeta {
		reTxt = regexp.QuoteMeta(q)
	}

	if len(flags) > 0 {
		reTxt = fmt.Sprintf("(?%s)%s", strings.Join(flags, ""), reTxt)
	}

	re, err := regexp.Compile(reTxt)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regular expression '%s': %w", reTxt, err)
	}
	return re, nil
}

// SplitQueryTerms splits a query string into positive and negative term slices.
// Terms starting with the negation prefix (followed by at least one more char)
// are negative (the prefix is stripped).
// Terms starting with a backslash followed by the prefix are positive literals
// (the backslash is stripped).
// A term that is exactly the prefix, or the prefix twice, is a positive literal.
// An empty prefix turns negative matching off: every term is positive, and
// backslashes are left alone.
// Empty tokens are skipped.
func SplitQueryTerms(query, negationPrefix string) (positive, negative []string) {
	tokens := strings.SplitSeq(strings.TrimSpace(query), " ")
	for tok := range tokens {
		if tok == "" {
			continue
		}
		if negationPrefix == "" {
			// Negative matching is off: everything is matched as typed
			positive = append(positive, tok)
			continue
		}
		if strings.HasPrefix(tok, `\`+negationPrefix) {
			// Escaped negative: treat as literal positive term (strip the backslash)
			positive = append(positive, tok[1:])
		} else if tok == negationPrefix || tok == negationPrefix+negationPrefix {
			// Bare prefix: literal positive
			positive = append(positive, tok)
		} else if strings.HasPrefix(tok, negationPrefix) {
			// Negative term: strip the prefix
			negative = append(negative, tok[len(negationPrefix):])
		} else {
			positive = append(positive, tok)
		}
	}
	return
}

// termsToRegexps compiles a slice of terms into regexps, using the full
// original query for flag computation (needed for SmartCase).
func termsToRegexps(terms []string, fullQuery string, flags regexpFlags, quotemeta bool) ([]*regexp.Regexp, error) {
	regexps := make([]*regexp.Regexp, 0, len(terms))
	for _, t := range terms {
		re, err := regexpFor(t, flags.flags(fullQuery), quotemeta)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regular expression '%s': %w", t, err)
		}
		regexps = append(regexps, re)
	}
	return regexps, nil
}

// newRegexpFilter is an internal helper that constructs a Regexp filter
// with the given name, flags, and quotemeta setting.
func newRegexpFilter(name string, flags regexpFlags, quotemeta bool, options []Option) *Regexp {
	rf := &Regexp{
		factory: &regexpQueryFactory{
			compiled:       make(map[string]regexpQuery),
			threshold:      time.Minute,
			negationPrefix: buildOptions(options).negationPrefix,
		},
		flags:     flags,
		quotemeta: quotemeta,
		name:      name,
	}
	rf.impl = rf
	return rf
}

// NewRegexp creates a new regexp based filter
func NewRegexp(options ...Option) *Regexp {
	return newRegexpFilter("Regexp", defaultFlags, false, options)
}

// NewIRegexp creates a new case-insensitive regexp based filter
func NewIRegexp(options ...Option) *Regexp {
	return newRegexpFilter("IRegexp", ignoreCaseFlags, false, options)
}

const maxRegexpCacheSize = 100

// evictLRU removes the least recently used entries from the cache until its
// size is at or below targetSize. The caller must hold f.mutex.
func (f *regexpQueryFactory) evictLRU(targetSize int) {
	type cacheEntry struct {
		key      string
		lastUsed time.Time
	}

	entries := make([]cacheEntry, 0, len(f.compiled))
	for k, v := range f.compiled {
		entries = append(entries, cacheEntry{key: k, lastUsed: v.lastUsed})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].lastUsed.Before(entries[j].lastUsed)
	})

	toRemove := len(f.compiled) - targetSize
	for i := range toRemove {
		delete(f.compiled, entries[i].key)
	}
}

// Compile parses the query string into positive and negative regexp slices,
// caching compiled results for reuse within the expiry threshold.
func (f *regexpQueryFactory) Compile(s string, flags regexpFlags, quotemeta bool) ([]*regexp.Regexp, []*regexp.Regexp, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	rq, ok := f.compiled[s]
	if ok {
		if time.Since(rq.lastUsed) < f.threshold {
			return rq.positive, rq.negative, nil
		}
		delete(f.compiled, s)
	}

	posTerms, negTerms := SplitQueryTerms(s, f.negationPrefix)

	var posRxs, negRxs []*regexp.Regexp
	if len(posTerms) > 0 {
		var err error
		posRxs, err = termsToRegexps(posTerms, s, flags, quotemeta)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to compile positive regular expressions: %w", err)
		}
	}
	if len(negTerms) > 0 {
		var err error
		negRxs, err = termsToRegexps(negTerms, s, flags, quotemeta)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to compile negative regular expressions: %w", err)
		}
	}

	// Evict stale entries if cache is over the size limit
	if len(f.compiled) >= maxRegexpCacheSize {
		for k, v := range f.compiled {
			if time.Since(v.lastUsed) >= f.threshold {
				delete(f.compiled, k)
			}
		}
		// If still over limit after evicting stale entries, evict the
		// least recently used entries to bring cache down to half capacity.
		if len(f.compiled) >= maxRegexpCacheSize {
			f.evictLRU(maxRegexpCacheSize / 2)
		}
	}

	rq.lastUsed = time.Now()
	rq.positive = posRxs
	rq.negative = negRxs
	f.compiled[s] = rq
	return posRxs, negRxs, nil
}

// applyInternal matches each line against the compiled positive and negative
// regexps, deduplicating overlapping match ranges before emitting results.
func (rf *Regexp) applyInternal(ctx context.Context, lines []line.Line, em LineEmitter) error {
	query := pipeline.QueryFromContext(ctx)
	posRegexps, negRegexps, err := rf.factory.Compile(query, rf.flags, rf.quotemeta)
	if err != nil {
		return fmt.Errorf("failed to compile queries as regular expression: %w", err)
	}

	// Pre-allocate match and dedup slices outside the loop so they can
	// be reused across lines, avoiding per-line heap allocations.
	var matches [][]int
	var deduped [][]int
	var flat []int

	lits := make([]literalMatcher, len(posRegexps))
	for i, rx := range posRegexps {
		lits[i] = newLiteralMatcher(rx)
	}

	for i, l := range lines {
		if err := checkCancelled(ctx, i); err != nil {
			return err
		}
		v := l.DisplayString()

		// Check negative terms first (fail-fast, no index collection)
		if isExcluded(negRegexps, v) {
			continue
		}

		// All-negative query: emit line with nil indices (no highlighting)
		if len(posRegexps) == 0 {
			em.Emit(ctx, line.GetMatched(l, nil))
			continue
		}

		// Positive matching (existing AND logic)
		allMatched := true
		matches = matches[:0]
		flat = flat[:0]
	TryRegexps:
		for ri, rx := range posRegexps {
			if lm := &lits[ri]; lm.ok {
				n := len(flat)
				flat = lm.appendAll(flat, v)
				if len(flat) == n {
					allMatched = false
					break TryRegexps
				}
				continue
			}
			match := rx.FindAllStringIndex(v, -1)
			if match == nil {
				allMatched = false
				break TryRegexps
			}
			matches = append(matches, match...)
		}

		if !allMatched {
			continue
		}

		for k := 0; k+1 < len(flat); k += 2 {
			matches = append(matches, flat[k:k+2:k+2])
		}

		slices.SortFunc(matches, func(a, b []int) int {
			if a[0] != b[0] {
				return a[0] - b[0]
			}
			return (a[1] - a[0]) - (b[1] - b[0])
		})

		// We need to "dedupe" the results. For example, if we matched the
		// same region twice, we don't want that to be drawn

		deduped = deduped[:0]

		for j, m := range matches {
			// Always push the first one
			if j == 0 {
				deduped = append(deduped, m)
				continue
			}

			prev := deduped[len(deduped)-1]
			switch {
			case matchContains(prev, m):
				// If the previous match contains this one, then
				// don't do anything
				continue
			case matchOverlaps(prev, m):
				// If the previous match overlaps with this one,
				// merge the results and make it a bigger one
				deduped[len(deduped)-1] = mergeMatches(prev, m)
			default:
				deduped = append(deduped, m)
			}
		}

		// Make a copy of deduped for emission since we reuse the slice
		result := make([][]int, len(deduped))
		pairs := make([]int, 2*len(deduped))
		for k, m := range deduped {
			pairs[2*k], pairs[2*k+1] = m[0], m[1]
			result[k] = pairs[2*k : 2*k+2 : 2*k+2]
		}
		em.Emit(ctx, line.GetMatched(l, result))
	}
	return nil
}

func (rf *Regexp) SupportsParallel() bool {
	return true
}

func (rf *Regexp) String() string {
	return rf.name
}

// NewIgnoreCase creates a case-insensitive literal string filter.
func NewIgnoreCase(options ...Option) *Regexp {
	return newRegexpFilter("IgnoreCase", ignoreCaseFlags, true, options)
}

// NewCaseSensitive creates a case-sensitive literal string filter.
func NewCaseSensitive(options ...Option) *Regexp {
	return newRegexpFilter("CaseSensitive", defaultFlags, true, options)
}

// NewSmartCase creates a filter that turns ON the ignore-case flag in the regexp
// if the query contains no upper-case character
func NewSmartCase(options ...Option) *Regexp {
	return newRegexpFilter("SmartCase", regexpFlagFunc(func(q string) []string {
		if util.ContainsUpper(q) {
			return defaultFlags
		}
		return []string{"i"}
	}), true, options)
}

// literalMatcher finds non-overlapping occurrences of a plain ASCII literal,
// either case-sensitively or ASCII-case-insensitively, producing the same
// ranges as the equivalent regexp without per-match allocations.
type literalMatcher struct {
	ok   bool
	fold bool
	lit  string // lower-cased when fold is set
}

func newLiteralMatcher(rx *regexp.Regexp) literalMatcher {
	if prefix, complete := rx.LiteralPrefix(); complete && prefix != "" {
		return literalMatcher{ok: true, lit: prefix}
	}
	src, ok := strings.CutPrefix(rx.String(), "(?i)")
	if !ok || src == "" {
		return literalMatcher{}
	}
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c >= '0' && c <= '9', c == ' ', c == '_':
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			// k and s have non-ASCII case-fold equivalents
			switch c {
			case 'k', 'K', 's', 'S':
				return literalMatcher{}
			}
		default:
			return literalMatcher{}
		}
	}
	return literalMatcher{ok: true, fold: true, lit: strings.ToLower(src)}
}

func (m *literalMatcher) appendAll(dst []int, v string) []int {
	n := len(m.lit)
	if !m.fold {
		pos := 0
		for {
			i := strings.Index(v[pos:], m.lit)
			if i < 0 {
				return dst
			}
			dst = append(dst, pos+i, pos+i+n)
			pos += i + n
		}
	}
	first := m.lit[0]
	for i := 0; i+n <= len(v); {
		if lower(v[i]) != first || !equalFoldASCII(v[i:i+n], m.lit) {
			i++
			continue
		}
		dst = append(dst, i, i+n)
		i += n
	}
	return dst
}

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

func equalFoldASCII(s, lit string) bool {
	for i := 0; i < len(lit); i++ {
		if lower(s[i]) != lit[i] {
			return false
		}
	}
	return true
}
