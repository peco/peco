package peco

type Filter struct {
	*Ctx
}

func (f *Filter) Loop() {
	defer f.ReleaseWaitGroup()

	for {
		select {
		case <-f.LoopCh():
			return
		case q := <-f.QueryCh():
			results := f.Matcher().Match(q, f.Buffer())
			f.selection.Clear()
			f.DrawMatches(results)
		}
	}
}
