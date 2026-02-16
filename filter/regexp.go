package filter

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/pkg/errors"
)

func (r regexpFlagList) flags(_ string) []string {
	return []string(r)
}

func (r regexpFlagFunc) flags(s string) []string {
	return r(s)
}

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
		return nil, errors.Wrapf(err, "failed to compile regular expression '%s'", reTxt)
	}
	return re, nil
}

func queryToRegexps(query string, flags regexpFlags, quotemeta bool) ([]*regexp.Regexp, error) {
	queries := strings.Split(strings.TrimSpace(query), " ")
	regexps := make([]*regexp.Regexp, 0)

	for _, q := range queries {
		re, err := regexpFor(q, flags.flags(query), quotemeta)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to compile regular expression '%s'", q)
		}
		regexps = append(regexps, re)
	}

	return regexps, nil
}

func (rf *Regexp) NewContext(ctx context.Context, query string) context.Context {
	return newContext(ctx, query)
}

// NewRegexp creates a new regexp based filter
func NewRegexp() *Regexp {
	return &Regexp{
		factory: &regexpQueryFactory{
			compiled:  make(map[string]regexpQuery),
			threshold: time.Minute,
		},
		flags:     regexpFlagList(defaultFlags),
		quotemeta: false,
		name:      "Regexp",
		outCh:     pipeline.ChanOutput(make(chan interface{})),
	}
}

// NewIRegexp creates a new case-insensitive regexp based filter
func NewIRegexp() *Regexp {
	return &Regexp{
		factory: &regexpQueryFactory{
			compiled:  make(map[string]regexpQuery),
			threshold: time.Minute,
		},
		flags:     regexpFlagList(regexpFlagList{"i"}),
		quotemeta: false,
		name:      "IRegexp",
		outCh:     pipeline.ChanOutput(make(chan interface{})),
	}
}

func (rf *Regexp) BufSize() int {
	return 0
}

func (rf *Regexp) OutCh() <-chan interface{} {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()
	return rf.outCh
}

const maxRegexpCacheSize = 100

func (f *regexpQueryFactory) Compile(s string, flags regexpFlags, quotemeta bool) ([]*regexp.Regexp, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	rq, ok := f.compiled[s]
	if ok {
		if time.Since(rq.lastUsed) < f.threshold {
			return rq.rx, nil
		}
		delete(f.compiled, s)
	}

	rxs, err := queryToRegexps(s, flags, quotemeta)
	if err != nil {
		return nil, errors.Wrap(err, `failed to compile regular expression`)
	}

	// Evict stale entries if cache is over the size limit
	if len(f.compiled) >= maxRegexpCacheSize {
		now := time.Now()
		for k, v := range f.compiled {
			if now.Sub(v.lastUsed) >= f.threshold {
				delete(f.compiled, k)
			}
		}
		// If still over limit after evicting stale entries, clear all
		if len(f.compiled) >= maxRegexpCacheSize {
			f.compiled = make(map[string]regexpQuery)
		}
	}

	rq.lastUsed = time.Now()
	rq.rx = rxs
	f.compiled[s] = rq
	return rxs, nil
}

func (rf *Regexp) applyInternal(ctx context.Context, lines []line.Line, emit func(line.Line)) error {
	query := ctx.Value(queryKey).(string)
	regexps, err := rf.factory.Compile(query, rf.flags, rf.quotemeta)
	if err != nil {
		return errors.Wrap(err, "failed to compile queries as regular expression")
	}

	for i, l := range lines {
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		v := l.DisplayString()
		allMatched := true
		matches := [][]int{}
	TryRegexps:
		for _, rx := range regexps {
			match := rx.FindAllStringSubmatchIndex(v, -1)
			if match == nil {
				allMatched = false
				break TryRegexps
			}
			matches = append(matches, match...)
		}

		if !allMatched {
			continue
		}

		sort.Sort(byMatchStart(matches))

		// We need to "dedupe" the results. For example, if we matched the
		// same region twice, we don't want that to be drawn

		deduped := make([][]int, 0, len(matches))

		for i, m := range matches {
			// Always push the first one
			if i == 0 {
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
		emit(line.NewMatched(l, deduped))
	}
	return nil
}

func (rf *Regexp) Apply(ctx context.Context, lines []line.Line, out pipeline.ChanOutput) error {
	return rf.applyInternal(ctx, lines, func(l line.Line) {
		out.Send(ctx, l)
	})
}

func (rf *Regexp) ApplyCollect(ctx context.Context, lines []line.Line) ([]line.Line, error) {
	result := make([]line.Line, 0, len(lines)/2)
	err := rf.applyInternal(ctx, lines, func(l line.Line) {
		result = append(result, l)
	})
	return result, err
}

func (rf *Regexp) SupportsParallel() bool {
	return true
}

func (rf *Regexp) String() string {
	return rf.name
}

func NewIgnoreCase() *Regexp {
	rf := NewRegexp()
	rf.flags = ignoreCaseFlags
	rf.quotemeta = true
	rf.name = "IgnoreCase"
	return rf
}

func NewCaseSensitive() *Regexp {
	rf := NewRegexp()
	rf.quotemeta = true
	rf.name = "CaseSensitive"
	return rf
}

// NewSmartCase creates a filter that turns ON the ignore-case flag in the regexp
// if the query contains no upper-case character
func NewSmartCase() *Regexp {
	rf := NewRegexp()
	rf.quotemeta = true
	rf.name = "SmartCase"
	rf.flags = regexpFlagFunc(func(q string) []string {
		if util.ContainsUpper(q) {
			return defaultFlags
		}
		return []string{"i"}
	})
	return rf
}
