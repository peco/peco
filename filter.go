package peco

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

	// previous holds a channel that can cancel the previous
	// query. This is used when multiple queries come in succession
	// and the previous query is discarded anyway
	var previous chan struct{}
	for {
		select {
		case <-f.LoopCh():
			return
		case q := <-f.QueryCh():
			if previous != nil {
				// Tell the previous query to stop
				previous <- struct{}{}
			}
			previous = make(chan struct{}, 1)

			f.statusMessage = "Running query..."
			f.DrawMatches(nil)
			go f.Work(previous, q)
		}
	}
}
