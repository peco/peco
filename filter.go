package peco

// Filter is responsible for the actual "grep" part of peco
type Filter struct {
	*Ctx
	jobs chan string
}

// Work is the actual work horse that that does the matching
// in a goroutine of its own. It wraps Matcher.Match().
func (f *Filter) Work(cancel chan struct{}, q HubReq) {
	defer q.Done()
	query := q.DataString()
	if query == "" {
		f.DrawMatches(nil)
		return
	}
	f.current = f.Matcher().Match(cancel, query, f.Buffer())
	f.SendStatusMsg("")
	f.selection.Clear()
	f.DrawMatches(nil)
}

// Loop keeps watching for incoming queries, and upon receiving
// a query, spawns a goroutine to do the heavy work. It also
// checks for previously running queries, so we can avoid
// running many goroutines doing the grep at the same time
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

			f.SendStatusMsg("Running query...")
			go f.Work(previous, q)
		}
	}
}
