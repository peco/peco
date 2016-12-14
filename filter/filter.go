package filter

import "context"

// newContext initializes the context so that it is suitable
// to be passed to `Run()`
func newContext(ctx context.Context, query string) context.Context {
	return context.WithValue(ctx, queryKey, query)
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
func matchContains(a []int, b []int) bool {
	return a[0] <= b[0] && a[1] >= b[1]
}

func matchOverlaps(a []int, b []int) bool {
	return a[0] <= b[0] && a[1] >= b[0] ||
		a[0] <= b[1] && a[1] >= b[1]
}

func mergeMatches(a []int, b []int) []int {
	ret := make([]int, 2)

	// Note: In practice this should never happen
	// because we're sorting by N[0] before calling
	// this routine, but for completeness' sake...
	if a[0] < b[0] {
		ret[0] = a[0]
	} else {
		ret[0] = b[0]
	}

	if a[1] < b[1] {
		ret[1] = b[1]
	} else {
		ret[1] = a[1]
	}
	return ret
}
