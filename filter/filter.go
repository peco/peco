package filter

import (
	"context"
	"regexp"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// Filter is the interface that all filter implementations must satisfy.
type Filter interface {
	Apply(context.Context, []line.Line, pipeline.ChanOutput) error
	BufSize() int
	NewContext(context.Context, string) context.Context
	String() string
	// SupportsParallel returns true if this filter can safely be invoked
	// concurrently on independent chunks of lines. Filters that require
	// global state across all lines (e.g. sorted output) should return false.
	SupportsParallel() bool
}

// Collector is an optional interface that filters can implement to return
// matched lines directly as a slice, bypassing channel-based output.
// This avoids per-chunk channel allocation and goroutine overhead in the
// parallel filter path.
type Collector interface {
	ApplyCollect(context.Context, []line.Line) ([]line.Line, error)
}

// isExcluded reports whether text matches any of the given negative regexps.
func isExcluded(negRegexps []*regexp.Regexp, text string) bool {
	for _, rx := range negRegexps {
		if rx.MatchString(text) {
			return true
		}
	}
	return false
}

// newContext initializes the context so that it is suitable
// to be passed to `Run()`
func newContext(ctx context.Context, query string) context.Context {
	return pipeline.NewQueryContext(ctx, query)
}

// checkCancelled returns ctx.Err() every 1000 iterations so that
// long-running filter loops can bail out promptly on cancellation
// without paying the cost of a channel receive on every single line.
func checkCancelled(ctx context.Context, i int) error {
	if i%1000 == 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

// sort related stuff
type byMatchStart [][]int

func (m byMatchStart) Len() int {
	return len(m)
}

func (m byMatchStart) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m byMatchStart) Less(i, j int) bool {
	if m[i][0] < m[j][0] {
		return true
	}

	if m[i][0] == m[j][0] {
		return m[i][1]-m[i][0] < m[j][1]-m[j][0]
	}

	return false
}

// matchContains reports whether match range a fully contains match range b.
func matchContains(a []int, b []int) bool {
	return a[0] <= b[0] && a[1] >= b[1]
}

// matchOverlaps reports whether two match ranges overlap.
func matchOverlaps(a []int, b []int) bool {
	return a[0] <= b[0] && a[1] >= b[0] ||
		a[0] <= b[1] && a[1] >= b[1]
}

// mergeMatches combines two overlapping match ranges into a single range
// spanning both. It mutates and returns a to avoid a heap allocation.
func mergeMatches(a []int, b []int) []int {
	a[0] = min(a[0], b[0])
	a[1] = max(a[1], b[1])
	return a
}
