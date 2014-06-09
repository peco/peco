package percol

import (
	"regexp"
)

type Filter struct {
	*Ctx
}

func (f *Filter) Loop() {
	f.AddWaitGroup()
	defer f.ReleaseWaitGroup()

	for {
		select {
		case <-f.LoopCh():
			return
		case q := <-f.QueryCh():
			results := []Match{}
			re, err := regexp.Compile(regexp.QuoteMeta(q))
			if err != nil {
				// Should display this at the bottom of the screen, but for now,
				// ignore it
				continue
			}

			for _, line := range f.Buffer() {
				ms := re.FindAllStringSubmatchIndex(line.line, 1)
				if ms == nil {
					continue
				}
				results = append(results, Match{line.line, ms})
			}

			f.query = []rune(q)
			f.DrawMatches(results)
		}
	}
}
