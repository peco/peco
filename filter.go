package peco

import (
	"errors"
	"regexp"
)

// Filter is responsible for the actual "grep" part of peco
type Filter struct {
	*Ctx
}

// Work is the actual work horse that that does the matching
// in a goroutine of its own. It wraps Matcher.Match().
func (f *Filter) Work(cancel chan struct{}, q HubReq) {
	tracer.Printf("Filter.Work: START\n")
	defer tracer.Printf("Filter.Work: END\n")
	defer q.Done()

	query := q.DataString()
	if query == "" {
		tracer.Printf("Filter.Work: Resetting activingLineBuffer")
		f.ResetActiveLineBuffer()
	} else {
		f.rawLineBuffer.Replay()

		filter := f.Filter().Clone()
		filter.SetQuery(query)
		tracer.Printf("Running %#v filter using query '%s'", filter, query)

		filter.Accept(f.rawLineBuffer)
		buf := NewRawLineBuffer()
		buf.Accept(filter)

		f.SetActiveLineBuffer(buf)
	}

	f.SendStatusMsg("")
	f.SelectionClear()
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

type Filterer interface {
	Filter(LineBuffer) LineBuffer
	Cancel()
}

type QueryFilterer interface {
	Pipeliner
	Filterer
	Clone() QueryFilterer
	Accept(Pipeliner)
	SetQuery(string)
	Name() string
}

type SelectionFilter struct {
	sel *Selection
}

func (sf SelectionFilter) Name() string {
	return "SelectionFilter"
}

func (sf SelectionFilter) Filter(in LineBuffer) LineBuffer {
	return nil
}

// PagingFilter filters out a new LineBuffer based on entries
// per page and the page number
type PagingFilter struct {
	perPage     int
	currentPage int
}

func (pf PagingFilter) Name() string {
	return "PagingFilter"
}

func (pf PagingFilter) Filter(in LineBuffer) LineBuffer {
	out := &FilteredLineBuffer{
		src:       in,
		selection: []int{},
	}

	s := pf.perPage * (pf.currentPage - 1)
	e := s + pf.perPage
	if s > in.Size() {
		return out
	}
	if e >= in.Size() {
		e = in.Size()
	}

	for i := s; i < e; i++ {
		out.SelectSourceLineAt(i)
	}
	return out
}

type RegexpFilter struct {
	simplePipeline
	compiledQuery []*regexp.Regexp
	flags         regexpFlags
	quotemeta     bool
	query         string
	name          string
}

func NewRegexpFilter() *RegexpFilter {
	return &RegexpFilter{}
}

func (rf RegexpFilter) Clone() QueryFilterer {
	return &RegexpFilter{
		simplePipeline{},
		nil,
		rf.flags,
		rf.quotemeta,
		rf.query,
		rf.name,
	}
}

func (rf *RegexpFilter) Accept(p Pipeliner) {
	cancelCh, incomingCh := p.Pipeline()
	rf.cancelCh = cancelCh
	rf.outputCh = make(chan Line)
	go acceptPipeline(cancelCh, incomingCh, rf.outputCh, rf.filter)
}

var ErrFilterDidNotMatch = errors.New("error: filter did not match against given line")

func (rf *RegexpFilter) filter(l Line) error {
	tracer.Printf("RegexpFilter.filter: START")
	defer tracer.Printf("RegexpFilter.filter: END")
	regexps, err := rf.getQueryAsRegexps()
	if err != nil {
		return err
	}
	v := l.DisplayString()
	allMatched := true
	matches := [][]int{}
TryRegexps:
	for _, rx := range regexps {
		tracer.Printf("RegexpFilter.filter: matching '%s' against '%s'", v, rx)
		match := rx.FindAllStringSubmatchIndex(v, -1)
		if match == nil {
			allMatched = false
			break TryRegexps
		}
		matches = append(matches, match...)
	}

	if !allMatched {
		return ErrFilterDidNotMatch
	}

	tracer.Printf("RegexpFilter.filter: line matched pattern\n")
	return nil
}

func (rf *RegexpFilter) getQueryAsRegexps() ([]*regexp.Regexp, error) {
	if q := rf.compiledQuery; q != nil {
		return q, nil
	}
	q, err := queryToRegexps(rf.flags, rf.quotemeta, rf.query)
	if err != nil {
		return nil, err
	}

	rf.compiledQuery = q
	return q, nil
}

func (rf *RegexpFilter) SetQuery(q string) {
	rf.query = q
	rf.compiledQuery = nil
}

func (rf RegexpFilter) Name() string {
	return rf.name
}

func (rf RegexpFilter) Filter(in LineBuffer) LineBuffer {
	out := &MatchFilteredLineBuffer{
		FilteredLineBuffer{
			src:       in,
			selection: []int{},
		},
		[][][]int{},
	}

	regexps, err := queryToRegexps(rf.flags, rf.quotemeta, rf.query)
	if err != nil {
		return out
	}

	for i := 0; i < in.Size(); i++ {
		// Process line by line, until we receive a quit request
		// or until we're done
		select {
		case <-rf.cancelCh:
			break
		default:
			l, err := in.LineAt(i)
			if err != nil {
				continue
			}

			v := l.DisplayString()
			allMatched := true
			matches := [][]int{}
		TryRegexps:
			for _, rx := range regexps {
				match := rx.FindAllStringSubmatchIndex(v, -1)
				if match == nil {
					allMatched = false
					break TryRegexps
				}
				matches = append(matches, match...)
			}

			if allMatched {
				out.SelectMatchedSourceLineAt(i, matches)
			}
		}
	}

	return out
}

type FilterSet struct {
	filters []QueryFilterer
	current int
}

func (fs *FilterSet) Size() int {
	return len(fs.filters)
}

func (fs *FilterSet) Add(qf QueryFilterer) {
	fs.filters = append(fs.filters, qf)
}

func (fs *FilterSet) Rotate() {
	fs.current++
	if fs.current >= len(fs.filters) {
		fs.current = 0
	}
}

var ErrFilterNotFound = errors.New("specified filter was not found")
func (fs *FilterSet) SetCurrentByName(name string) error {
	for i, f := range fs.filters {
		if f.Name() == name {
			fs.current = i
			return nil
		}
	}
	return ErrFilterNotFound
}

func (fs *FilterSet) GetCurrent() QueryFilterer {
	return fs.filters[fs.current]
}

func NewIgnoreCaseFilter() *RegexpFilter {
	return &RegexpFilter{
		flags:     regexpFlagList(ignoreCaseFlags),
		quotemeta: true,
		name:      "IgnoreCase",
	}
}

func NewCaseSensitiveFilter() *RegexpFilter {
	return &RegexpFilter{
		flags:     regexpFlagList(defaultFlags),
		quotemeta: true,
		name:      "CaseSensitive",
	}
}
