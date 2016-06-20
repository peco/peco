package peco

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

var ignoreCaseFlags = regexpFlagList([]string{"i"})
var defaultFlags = regexpFlagList{}

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

	if flags != nil && len(flags) > 0 {
		reTxt = fmt.Sprintf("(?%s)%s", strings.Join(flags, ""), reTxt)
	}

	re, err := regexp.Compile(reTxt)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compile regular expression '%s'", reTxt)
	}
	return re, nil
}

func queryToRegexps(flags regexpFlags, quotemeta bool, query string) ([]*regexp.Regexp, error) {
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
