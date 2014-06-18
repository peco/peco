package peco

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Match defines the interface for matches. Note that to make drawing easier,
// we have a DidMatch and NoMatch types instead of using []Match and []string.
type Match interface {
	Buffer() string // Raw buffer, may contain null
	Line() string // Line to be displayed
	Output() string // Output string to be displayed after peco is done
	Indices() [][]int
}

type MatchString struct {
	buf string
	sepLoc int
}

func NewMatchString(v string, enableSep bool) *MatchString {
	m := &MatchString{
		v,
		-1,
	}
	if !enableSep {
		return m
	}

	if i := strings.IndexByte(string(m.buf), '\000'); i > -1 {
		m.sepLoc = i
	}
	return m
}

func (m MatchString) Buffer() string {
	return m.buf
}

func (m MatchString) Line() string {
	if i := m.sepLoc; i > -1 {
		return m.buf[:i]
	}
	return m.buf
}

func (m MatchString) Output() string {
	if i := m.sepLoc; i > -1 {
		return m.buf[i:]
	}
	return m.buf
}

// NoMatch is actually an alias to a regular string. It implements the
// Match interface, but just returns the underlying string with no matches
type NoMatch struct {
	*MatchString
}

func NewNoMatch(v string, enableSep bool) *NoMatch {
	return &NoMatch{NewMatchString(v, enableSep)}
}

func (m NoMatch) Indices() [][]int {
	return nil
}

// DidMatch contains the actual match, and the indices to the matches 
// in the line
type DidMatch struct {
	*MatchString
	matches [][]int
}

func NewDidMatch(v string, enableSep bool, m [][]int) *DidMatch {
	return &DidMatch{NewMatchString(v, enableSep), m}
}

func (d DidMatch) Indices() [][]int {
	return d.matches
}

// Matcher interface defines the API for things that want to
// match against the buffer
type Matcher interface {
	Match(string, []Match) []Match
	String() string
}

const (
	IgnoreCaseMatch    = "IgnoreCase"
	CaseSensitiveMatch = "CaseSensitive"
	RegexpMatch        = "Regexp"
)

type RegexpMatcher struct {
	enableSep bool
	flags     []string
	quotemeta bool
}

type CaseSensitiveMatcher struct {
	*RegexpMatcher
}

type IgnoreCaseMatcher struct {
	*RegexpMatcher
}

func NewCaseSensitiveMatcher(enableSep bool) *CaseSensitiveMatcher {
	m := &CaseSensitiveMatcher{NewRegexpMatcher(enableSep)}
	m.quotemeta = true
	return m
}

func NewIgnoreCaseMatcher(enableSep bool) *IgnoreCaseMatcher {
	m := &IgnoreCaseMatcher{NewRegexpMatcher(enableSep)}
	m.flags = []string{"i"}
	m.quotemeta = true
	return m
}

func NewRegexpMatcher(enableSep bool) *RegexpMatcher {
	return &RegexpMatcher{
		enableSep,
		[]string{},
		false,
	}
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
		return nil, err
	}
	return re, nil
}

func (m *RegexpMatcher) QueryToRegexps(query string) ([]*regexp.Regexp, error) {
	queries := strings.Split(strings.TrimSpace(query), " ")
	regexps := make([]*regexp.Regexp, 0)

	for _, q := range queries {
		re, err := regexpFor(q, m.flags, m.quotemeta)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, re)
	}

	return regexps, nil
}

func (m *RegexpMatcher) String() string {
	return "Regexp"
}

func (m *CaseSensitiveMatcher) String() string {
	return "CaseSensitive"
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

func (m *RegexpMatcher) Match(q string, buffer []Match) []Match {
	results := []Match{}
	regexps, err := m.QueryToRegexps(q)
	if err != nil {
		return results
	}

	for _, line := range buffer {
		ms := m.MatchAllRegexps(regexps, line.Line())
		if ms == nil {
			continue
		}
		results = append(results, NewDidMatch(line.Buffer(), m.enableSep, ms))
	}
	return results
}

func (m *RegexpMatcher) MatchAllRegexps(regexps []*regexp.Regexp, line string) [][]int {
	matches := make([][]int, 0)

	allMatched := true
Match:
	for _, re := range regexps {
		match := re.FindAllStringSubmatchIndex(line, -1)
		if match == nil {
			allMatched = false
			break Match
		}

		for _, ma := range match {
			start, end := ma[0], ma[1]
			for _, m := range matches {
				if start >= m[0] && start < m[1] {
					continue Match
				}

				if start < m[0] && end >= m[0] {
					continue Match
				}
			}
			matches = append(matches, ma)
		}
	}

	if !allMatched {
		return nil
	}

	sort.Sort(byStart(matches))

	return matches
}
