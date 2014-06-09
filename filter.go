package main

import (
	"regexp"
	"sync"
)

type Filter struct {
	queryCh chan string
	wait    *sync.WaitGroup
}

func (f *Filter) Execute(v string) {
	f.queryCh <- v
}

func (f *Filter) Loop() {
	f.wait.Add(1)
	defer f.wait.Done()

	for {
		select {
		case <-ctx.loopCh:
			return
		case q := <-f.queryCh:
			results := []Match{}
			re, err := regexp.Compile(regexp.QuoteMeta(q))
			if err != nil {
				// Should display this at the bottom of the screen, but for now,
				// ignore it
				continue
			}

			// XXX accessing ctx.lines is bad idea
			for _, line := range ctx.lines {
				ms := re.FindAllStringSubmatchIndex(line.line, 1)
				if ms == nil {
					continue
				}
				results = append(results, Match{line.line, ms})
			}

			ui.DrawMatches(results)
		}
	}
}
