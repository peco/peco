package peco

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	CaseSensitiveMatch = iota
	IgnoreCaseMatch
)

type StraightMatcher struct {
	flags []string
}

type CaseSensitiveMatcher struct {
	StraightMatcher
}

type IgnoreCaseMatcher struct {
	StraightMatcher
}

func NewCaseSensitiveMatcher() *CaseSensitiveMatcher {
	return &CaseSensitiveMatcher{StraightMatcher{nil}}
}

func NewIgnoreCaseMatcher() *IgnoreCaseMatcher {
	return &IgnoreCaseMatcher{StraightMatcher{[]string{"i"}}}
}

func regexpFor(q string, flags []string) (*regexp.Regexp, error) {
	var reTxt string
	if flags == nil || len(flags) <= 0 {
		reTxt = fmt.Sprintf("%s", regexp.QuoteMeta(q))
	} else {
		reTxt = fmt.Sprintf("(?%s)%s", strings.Join(flags, ""), regexp.QuoteMeta(q))
	}
	re, err := regexp.Compile(reTxt)
	if err != nil {
		return nil, err
	}
	return re, nil
}

func (m *StraightMatcher) QueryToRegexps(query string) ([]*regexp.Regexp, error) {
	queries := strings.Split(strings.TrimSpace(query), " ")
	regexps := make([]*regexp.Regexp, 0)

	for _, q := range queries {
		re, err := regexpFor(q, m.flags)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, re)
	}

	return regexps, nil
}

func (m *CaseSensitiveMatcher) String() string {
	return "CaseSentive"
}

func (m *IgnoreCaseMatcher) String() string {
	return "IgnoreCase"
}

// sort related stuff
type byStart [][]int

func (m byStart) Len() int {
	return len(m)
}

func (m byStart) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m byStart) Less(i, j int) bool {
	return m[i][0] < m[j][0]
}

func (m *StraightMatcher) Match(q string, buffer []Match) []Match {
	results := []Match{}
	regexps, err := m.QueryToRegexps(q)
	if err != nil {
		return []Match{}
	}

	for _, line := range buffer {
		ms := m.MatchAllRegexps(regexps, line.line)
		if ms == nil {
			continue
		}
		results = append(results, Match{line.line, ms})
	}
	return results
}

func (m *StraightMatcher) MatchAllRegexps(regexps []*regexp.Regexp, line string) [][]int {
	matches := make([][]int, 0)

	allMatched := true
Match:
	for _, re := range regexps {
		match := re.FindAllStringSubmatchIndex(line, 1)
		if match == nil {
			allMatched = false
			break Match
		}

		start, end := match[0][0], match[0][1]
		for _, m := range matches {
			if start >= m[0] && start < m[1] {
				continue Match
			}

			if start < m[0] && end >= m[0] {
				continue Match
			}
		}

		matches = append(matches, match[0])
		sort.Sort(byStart(matches))
	}

	if !allMatched {
		return nil
	}

	return matches
}
