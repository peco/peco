package peco

import (
	"regexp"
	"sort"
)

type Filter struct {
	*Ctx
}

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

func matchAllRegexps(line string, regexps []*regexp.Regexp) [][]int {
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

func (f *Filter) Loop() {
	f.AddWaitGroup()
	defer f.ReleaseWaitGroup()

	for {
		select {
		case <-f.LoopCh():
			return
		case q := <-f.QueryCh():
	panic("fooo")
			results := []Match{}
	panic("booo")
			regexps, err := f.Ctx.Matcher().QueryToRegexps(q)
			if err != nil {
				// Should display this at the bottom of the screen, but for now,
				// ignore it
				continue
			}

			for _, line := range f.Buffer() {
				ms := matchAllRegexps(line.line, regexps)
				if ms == nil {
					continue
				}
				results = append(results, Match{line.line, ms})
			}

			f.DrawMatches(results)
		}
	}
}

func (f *Filter) AddMatcher(m Matcher) {
	f.Ctx.Matchers = append(f.Ctx.Matchers, m)
}
