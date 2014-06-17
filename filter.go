package peco

import (
	"fmt"
	"os"
	"time"
)

type Filter struct {
	*Ctx
	jobs chan string
}

func (f *Filter) Work(cancel chan struct{}, q string) {
	if q == "" {
		f.DrawMatches(nil)
		return
	}
	results := f.Matcher().Match(cancel, q, f.Buffer())
	f.statusMessage = ""
	f.selection.Clear()
	f.DrawMatches(results)
}

func (f *Filter) Loop() {
	defer f.ReleaseWaitGroup()

	var previous chan struct{}
	for {
		select {
		case <-f.LoopCh():
			return
		case q := <-f.QueryCh():
			// Stop all workers
			if previous != nil {
				previous <- struct{}{}
			}
			previous = make(chan struct{}, 1)

			f.statusMessage = "Running query..."
			f.DrawMatches(nil)
			go f.Work(previous, q)
		}
	}
}
