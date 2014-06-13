package peco

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
			results := f.Matcher().Match(q, f.Buffer())
			f.DrawMatches(results)
		}
	}
}

func (f *Filter) AddMatcher(m Matcher) {
	f.Ctx.Matchers = append(f.Ctx.Matchers, m)
}
