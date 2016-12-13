package peco

import (
	"bufio"
	"bytes"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lestrrat/go-pdebug"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/pipeline"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func (fs *FilterSet) Reset() {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.current = 0
}

func (fs *FilterSet) Size() int {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return len(fs.filters)
}

func (fs *FilterSet) Add(lf LineFilter) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.filters = append(fs.filters, lf)
	return nil
}

func (fs *FilterSet) Rotate() {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	fs.current++
	if fs.current >= len(fs.filters) {
		fs.current = 0
	}
	if pdebug.Enabled {
		pdebug.Printf("FilterSet.Rotate: now filter in effect is %s", fs.filters[fs.current])
	}
}

func (fs *FilterSet) SetCurrentByName(name string) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	for i, f := range fs.filters {
		if f.String() == name {
			fs.current = i
			return nil
		}
	}
	return ErrFilterNotFound
}

func (fs *FilterSet) Index() int {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return fs.current
}

func (fs *FilterSet) Current() LineFilter {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	return fs.filters[fs.current]
}

func NewFilter(state *Peco) *Filter {
	return &Filter{
		state: state,
	}
}

// Work is the actual work horse that that does the matching
// in a goroutine of its own. It wraps Matcher.Match().
func (f *Filter) Work(ctx context.Context, q hub.Payload) {
	defer q.Done()

	query, ok := q.Data().(string)
	if !ok {
		return
	}

	if pdebug.Enabled {
		g := pdebug.Marker("Filter.Work query '%s'", query)
		defer g.End()
	}

	state := f.state
	if query == "" {
		state.ResetCurrentLineBuffer()
		if !state.config.StickySelection {
			state.Selection().Reset()
		}
		return
	}

	// Create a new pipeline
	p := pipeline.New()
	p.SetSource(state.Source())
	thisf := state.Filters().Current().Clone()
	thisf.SetQuery(query)
	p.Add(thisf)

	buf := NewMemoryBuffer()
	p.SetDestination(buf)
	state.SetCurrentLineBuffer(buf)

	go func() {
		defer state.Hub().SendDraw(&DrawOptions{RunningQuery: true})
		ctx = context.WithValue(ctx, "query", query)
		if err := p.Run(ctx); err != nil {
			state.Hub().SendStatusMsg(err.Error())
		}
	}()

	go func() {
		if pdebug.Enabled {
			g := pdebug.Marker("Periodic draw request for '%s'", query)
			defer g.End()
		}
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		defer state.Hub().SendStatusMsg("")
		defer state.Hub().SendDraw(&DrawOptions{RunningQuery: true})
		for {
			select {
			case <-p.Done():
				return
			case <-t.C:
				state.Hub().SendDraw(&DrawOptions{RunningQuery: true})
			}
		}
	}()

	<-p.Done()

	if !state.config.StickySelection {
		state.Selection().Reset()
	}
}

// Loop keeps watching for incoming queries, and upon receiving
// a query, spawns a goroutine to do the heavy work. It also
// checks for previously running queries, so we can avoid
// running many goroutines doing the grep at the same time
func (f *Filter) Loop(ctx context.Context, cancel func()) error {
	defer cancel()

	// previous holds the function that can cancel the previous
	// query. This is used when multiple queries come in succession
	// and the previous query is discarded anyway
	var mutex sync.Mutex
	var previous func()
	for {
		select {
		case <-ctx.Done():
			return nil
		case q := <-f.state.Hub().QueryCh():
			workctx, workcancel := context.WithCancel(ctx)

			mutex.Lock()
			if previous != nil {
				if pdebug.Enabled {
					pdebug.Printf("Canceling previous query")
				}
				previous()
			}
			previous = workcancel
			mutex.Unlock()

			f.state.Hub().SendStatusMsg("Running query...")

			go f.Work(workctx, q)
		}
	}
}

func NewRegexpFilter() *RegexpFilter {
	return &RegexpFilter{
		flags: regexpFlagList(defaultFlags),
		name:  "Regexp",
		outCh: pipeline.OutputChannel(make(chan interface{})),
	}
}

func (rf *RegexpFilter) OutCh() <-chan interface{} {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()
	return rf.outCh
}

func (rf RegexpFilter) Clone() LineFilter {
	return &RegexpFilter{
		flags:     rf.flags,
		quotemeta: rf.quotemeta,
		query:     rf.query,
		name:      rf.name,
		outCh:     pipeline.OutputChannel(make(chan interface{})),
	}
}

const filterBufSize = 1000

var filterBufPool = sync.Pool{
	New: func() interface{} {
		return make([]Line, 0, filterBufSize)
	},
}

func releaseFilterLineBuf(l []Line) {
	if l == nil {
		return
	}
	l = l[0:0]
	filterBufPool.Put(l)
}

func getFilterLineBuf() []Line {
	l := filterBufPool.Get().([]Line)
	return l
}

type filter interface {
	filter(Line) (Line, error)
}

// This flusher is run in a separate goroutine so that the filter can
// run separately from accepting incoming messages
func flusher(f filter, incoming chan []Line, done chan struct{}, out pipeline.OutputChannel) {
	if pdebug.Enabled {
		g := pdebug.Marker("flusher goroutine")
		defer g.End()
	}

	defer close(done)
	defer out.SendEndMark("end of filter")
	for buf := range incoming {
		for _, in := range buf {
			if l, err := f.filter(in); err == nil {
				out.Send(l)
			}
		}
		releaseFilterLineBuf(buf)
	}
}

func (rf *RegexpFilter) Accept(ctx context.Context, in chan interface{}, out pipeline.OutputChannel) {
	if pdebug.Enabled {
		g := pdebug.Marker("RegexpFilter.Accept")
		defer g.End()
	}

	filterAcceptAndFilter(ctx, rf, in, out)
}

func filterAcceptAndFilter(ctx context.Context, f filter, in chan interface{}, out pipeline.OutputChannel) {
	flush := make(chan []Line)
	flushDone := make(chan struct{})
	go flusher(f, flush, flushDone, out)

	buf := getFilterLineBuf()
	defer releaseFilterLineBuf(buf)
	defer func() { <-flushDone }() // Wait till the flush goroutine is done
	defer close(flush)             // Kill the flush goroutine

	flushTicker := time.NewTicker(50 * time.Millisecond)
	defer flushTicker.Stop()

	start := time.Now()
	lines := 0
	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("filter received done")
			}
			return
		case v := <-in:
			switch v.(type) {
			case error:
				if pipeline.IsEndMark(v.(error)) {
					if pdebug.Enabled {
						pdebug.Printf("filter received end mark (read %d lines, %s since starting accept loop)", lines+len(buf), time.Since(start).String())
					}
					if len(buf) > 0 {
						flush <- buf
						buf = nil
					}
				}
				return
			case Line:
				if pdebug.Enabled {
					lines++
				}
				// We buffer the lines so that we can receive more lines to
				// process while we filter what we already have. The buffer
				// size is fairly big, because this really only makes a
				// difference if we have a lot of lines to process.
				buf = append(buf, v.(Line))
				select {
				case <-flushTicker.C:
					flush <- buf
					buf = getFilterLineBuf()
				default:
					if len(buf) >= cap(buf) {
						flush <- buf
						buf = getFilterLineBuf()
					}
				}
			}
		}
	}
}

func (rf *RegexpFilter) filter(l Line) (Line, error) {
	regexps, err := rf.getQueryAsRegexps()
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile queries as regular expression")
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

	if !allMatched {
		return nil, errors.New("filter did not match against given line")
	}

	sort.Sort(byMatchStart(matches))

	// We need to "dedupe" the results. For example, if we matched the
	// same region twice, we don't want that to be drawn

	deduped := make([][]int, 0, len(matches))

	for i, m := range matches {
		// Always push the first one
		if i == 0 {
			deduped = append(deduped, m)
			continue
		}

		prev := deduped[len(deduped)-1]
		switch {
		case matchContains(prev, m):
			// If the previous match contains this one, then
			// don't do anything
			continue
		case matchOverlaps(prev, m):
			// If the previous match overlaps with this one,
			// merge the results and make it a bigger one
			deduped[len(deduped)-1] = mergeMatches(prev, m)
		default:
			deduped = append(deduped, m)
		}
	}
	return NewMatchedLine(l, deduped), nil
}

func (rf *RegexpFilter) getQueryAsRegexps() ([]*regexp.Regexp, error) {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	if q := rf.compiledQuery; q != nil {
		return q, nil
	}
	q, err := queryToRegexps(rf.flags, rf.quotemeta, rf.query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile queries as regular expression")
	}

	rf.compiledQuery = q
	return q, nil
}

func (rf *RegexpFilter) SetQuery(q string) {
	rf.mutex.Lock()
	defer rf.mutex.Unlock()

	rf.query = q
	rf.compiledQuery = nil
}

func (rf RegexpFilter) String() string {
	return rf.name
}

var ErrFilterNotFound = errors.New("specified filter was not found")

func NewIgnoreCaseFilter() *RegexpFilter {
	rf := NewRegexpFilter()
	rf.flags = ignoreCaseFlags
	rf.quotemeta = true
	rf.name = "IgnoreCase"
	return rf
}

func NewCaseSensitiveFilter() *RegexpFilter {
	rf := NewRegexpFilter()
	rf.quotemeta = true
	rf.name = "CaseSensitive"
	return rf
}

// SmartCaseFilter turns ON the ignore-case flag in the regexp
// if the query contains a upper-case character
func NewSmartCaseFilter() *RegexpFilter {
	rf := NewRegexpFilter()
	rf.quotemeta = true
	rf.name = "SmartCase"
	rf.flags = regexpFlagFunc(func(q string) []string {
		if util.ContainsUpper(q) {
			return defaultFlags
		}
		return []string{"i"}
	})
	return rf
}

// NewFuzzyFilter builds a fuzzy-finder type of filter.
// In effect, this uses a smart case filter, and for q query 
// like "ABC" it matches the equivalent of "A(.*)B(.*)C(.*)"
func NewFuzzyFilter() *FuzzyFilter {
	return &FuzzyFilter{}
}

func (ff FuzzyFilter) Clone() LineFilter {
	return &FuzzyFilter{
		query: ff.query,
	}
}

func (ff *FuzzyFilter) SetQuery(q string) {
	ff.mutex.Lock()
	defer ff.mutex.Unlock()

	ff.query = q
}

func (ff FuzzyFilter) String() string {
	return "Fuzzy"
}

func (ff *FuzzyFilter) Accept(ctx context.Context, in chan interface{}, out pipeline.OutputChannel) {
	if pdebug.Enabled {
		g := pdebug.Marker("FuzzyFilter.Accept")
		defer g.End()
	}

	filterAcceptAndFilter(ctx, ff, in, out)
}

func (ff *FuzzyFilter) filter(l Line) (Line, error) {
	query := ""
	ff.mutex.Lock()
	query = ff.query
	ff.mutex.Unlock()

	base := 0
	txt := l.DisplayString()
	matches := [][]int{}

	hasUpper := util.ContainsUpper(query)

	for len(query) > 0 {
		r, n := utf8.DecodeRuneInString(query)
		if r == utf8.RuneError {
			// "Silently" ignore (just return a no match)
			return nil, errors.New("failed to decode input string")
		}
		query = query[n:]

		var i int
		if hasUpper { // explicit match
			i = strings.IndexRune(txt, r)
		} else {
			i = strings.IndexFunc(txt, util.CaseInsensitiveIndexFunc(r))
		}
		if i == -1 {
			return nil, errors.New("filter did not match against given line")
		}

		// otherwise we have a match, but the next match must match against
		// something AFTER the current match
		txt = txt[i+n:]
		matches = append(matches, []int{base + i, base + i + n})
		base = base + i + n
	}
	return NewMatchedLine(l, matches), nil
}

func NewExternalCmdFilter(name string, cmd string, args []string, threshold int, idgen lineIDGenerator, enableSep bool) *ExternalCmdFilter {
	if len(args) == 0 {
		args = []string{"$QUERY"}
	}

	if threshold <= 0 {
		threshold = DefaultCustomFilterBufferThreshold
	}

	return &ExternalCmdFilter{
		args:            args,
		cmd:             cmd,
		enableSep:       enableSep,
		idgen:           idgen,
		name:            name,
		outCh:           pipeline.OutputChannel(make(chan interface{})),
		thresholdBufsiz: threshold,
	}
}

func (ecf ExternalCmdFilter) Clone() LineFilter {
	return &ExternalCmdFilter{
		args:            ecf.args,
		cmd:             ecf.cmd,
		enableSep:       ecf.enableSep,
		idgen:           ecf.idgen,
		name:            ecf.name,
		outCh:           pipeline.OutputChannel(make(chan interface{})),
		thresholdBufsiz: ecf.thresholdBufsiz,
	}
}

func (ecf *ExternalCmdFilter) Verify() error {
	if ecf.cmd == "" {
		return errors.Errorf("no executable specified for custom matcher '%s'", ecf.name)
	}

	if _, err := exec.LookPath(ecf.cmd); err != nil {
		return errors.Wrap(err, "failed to locate command")
	}
	return nil
}

func (ecf *ExternalCmdFilter) Accept(ctx context.Context, in chan interface{}, out pipeline.OutputChannel) {
	if pdebug.Enabled {
		g := pdebug.Marker("ExternalCmdFilter.Accept")
		defer g.End()
	}
	defer out.SendEndMark("end of ExternalCmdFilter")

	buf := make([]Line, 0, ecf.thresholdBufsiz)
	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("ExternalCmdFilter received done")
			}
			return
		case v := <-in:
			switch v.(type) {
			case error:
				if pipeline.IsEndMark(v.(error)) {
					if pdebug.Enabled {
						pdebug.Printf("ExternalCmdFilter received end mark")
					}
					if len(buf) > 0 {
						ecf.launchExternalCmd(ctx, buf, out)
					}
				}
				return
			case Line:
				if pdebug.Enabled {
					pdebug.Printf("ExternalCmdFilter received new line")
				}
				buf = append(buf, v.(Line))
				if len(buf) < ecf.thresholdBufsiz {
					continue
				}

				ecf.launchExternalCmd(ctx, buf, out)
				buf = buf[0:0]
			}
		}
	}
}

func (ecf *ExternalCmdFilter) SetQuery(q string) {
	ecf.query = q
}

func (ecf ExternalCmdFilter) String() string {
	return ecf.name
}

func (ecf *ExternalCmdFilter) launchExternalCmd(ctx context.Context, buf []Line, out pipeline.OutputChannel) {
	defer func() { recover() }() // ignore errors
	if pdebug.Enabled {
		g := pdebug.Marker("ExternalCmdFilter.launchExternalCmd")
		defer g.End()
	}

	args := append([]string(nil), ecf.args...)
	for i, v := range args {
		if v == "$QUERY" {
			args[i] = ecf.query
		}
	}
	cmd := exec.Command(ecf.cmd, args...)
	if pdebug.Enabled {
		pdebug.Printf("Executing command %s %v", cmd.Path, cmd.Args)
	}

	inbuf := &bytes.Buffer{}
	for _, l := range buf {
		inbuf.WriteString(l.DisplayString() + "\n")
	}

	cmd.Stdin = inbuf
	r, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	err = cmd.Start()
	if err != nil {
		return
	}

	go cmd.Wait()

	cmdCh := make(chan Line)
	go func(cmdCh chan Line, rdr *bufio.Reader) {
		defer func() { recover() }()
		defer close(cmdCh)
		for {
			b, _, err := rdr.ReadLine()
			if len(b) > 0 {
				// TODO: need to redo the spec for custom matchers
				// This is the ONLY location where we need to actually
				// RECREATE a RawLine, and thus the only place where
				// ctx.enableSep is required.
				cmdCh <- NewMatchedLine(NewRawLine(ecf.idgen.next(), string(b), ecf.enableSep), nil)
			}
			if err != nil {
				break
			}
		}
	}(cmdCh, bufio.NewReader(r))

	defer func() {
		if p := cmd.Process; p != nil {
			p.Kill()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case l, ok := <-cmdCh:
			if l == nil || !ok {
				return
			}
			out.Send(l)
		}
	}
}
