package peco

import (
	"bufio"
	"bytes"
	"os/exec"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/lestrrat/go-pdebug"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/pipeline"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func (fx *FilterSet) Reset() {
	fx.current = 0
}

func (fs *FilterSet) Size() int {
	return len(fs.filters)
}

func (fs *FilterSet) Add(lf LineFilter) error {
	fs.filters = append(fs.filters, lf)
	return nil
}

func (fs *FilterSet) Rotate() {
	fs.current++
	if fs.current >= len(fs.filters) {
		fs.current = 0
	}
	if pdebug.Enabled {
		pdebug.Printf("FilterSet.Rotate: now filter in effect is %s", fs.filters[fs.current])
	}
}

func (fs *FilterSet) SetCurrentByName(name string) error {
	for i, f := range fs.filters {
		if f.String() == name {
			fs.current = i
			return nil
		}
	}
	return ErrFilterNotFound
}

func (fs *FilterSet) Current() LineFilter {
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
		pdebug.Printf("New query '%s'", query)
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
		defer state.Hub().SendDraw(true)
		if err := p.Run(ctx); err != nil {
			state.Hub().SendStatusMsg(err.Error())
		}
	}()

	go func() {
		if pdebug.Enabled {
			pdebug.Printf("waiting for query to finish")
			defer pdebug.Printf("Filter.Work: finished running query")
		}
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-p.Done():
				break
			case <-t.C:
				state.Hub().SendDraw(true)
			}
		}
		state.Hub().SendStatusMsg("")
	}()

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
	previous = func() {} // no op func
	for {
		select {
		case <-ctx.Done():
			return nil
		case q := <-f.state.Hub().QueryCh():
			workctx, workcancel := context.WithCancel(ctx)

			mutex.Lock()
			previous()
			previous = workcancel
			mutex.Unlock()

			f.state.Hub().SendStatusMsg("Running query...")

			go func() {
				f.Work(workctx, q)
			}()
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

func (rf RegexpFilter) OutCh() <-chan interface{} {
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

func releaseRegexpFilterBuf(l []Line) {
	if l == nil {
		return
	}
	l = l[0:0]
	filterBufPool.Put(l)
}

func getRegexpFilterBuf() []Line {
	l := filterBufPool.Get().([]Line)
	return l
}

func (rf *RegexpFilter) Accept(ctx context.Context, p pipeline.Producer) {
	if pdebug.Enabled {
		g := pdebug.Marker("RegexpFilter.Accept")
		defer g.End()
	}
	defer rf.outCh.SendEndMark("end of RegexpFilter")

	flush := make(chan []Line)
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		for buf := range flush {
			for _, in := range buf {
				if l, err := rf.filter(in); err == nil {
					rf.outCh.Send(l)
				}
			}
			releaseRegexpFilterBuf(buf)
		}
	}()

	buf := getRegexpFilterBuf()
	defer func() { releaseRegexpFilterBuf(buf) }()
	defer func() { <-flushDone }() // Wait till the flush goroutine is done
	defer close(flush) // Kill the flush goroutine

	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("RegexpFilter received done")
			}
			return
		case v := <-p.OutCh():
			switch v.(type) {
			case error:
				if pipeline.IsEndMark(v.(error)) {
					if pdebug.Enabled {
						pdebug.Printf("RegexpFilter received end mark")
					}
					if len(buf) > 0 {
						flush <-buf
						buf = nil
					}
				}
				return
			case Line:
				buf = append(buf, v.(Line))
				if len(buf) >= cap(buf) {
					flush <- buf
					buf = getRegexpFilterBuf()
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

func NewExternalCmdFilter(name string, cmd string, args []string, threshold int, enableSep bool) *ExternalCmdFilter {
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

func (ecf *ExternalCmdFilter) Accept(ctx context.Context, p pipeline.Producer) {
	if pdebug.Enabled {
		g := pdebug.Marker("ExternalCmdFilter.Accept")
		defer g.End()
	}
	defer ecf.outCh.SendEndMark("end of ExternalCmdFilter")

	buf := make([]Line, 0, ecf.thresholdBufsiz)
	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("ExternalCmdFilter received done")
			}
			return
		case v := <-p.OutCh():
			switch v.(type) {
			case error:
				if pipeline.IsEndMark(v.(error)) {
					if pdebug.Enabled {
						pdebug.Printf("ExternalCmdFilter received end mark")
					}
					if len(buf) > 0 {
						ecf.launchExternalCmd(ctx, buf)
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

				ecf.launchExternalCmd(ctx, buf)
				buf = buf[0:0]
			}
		}
	}
}

func (ecf ExternalCmdFilter) OutCh() <-chan interface{} {
	return ecf.outCh
}

func (ecf *ExternalCmdFilter) SetQuery(q string) {
	ecf.query = q
}

func (ecf ExternalCmdFilter) String() string {
	return ecf.name
}

func (ecf *ExternalCmdFilter) launchExternalCmd(ctx context.Context, buf []Line) {
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
				cmdCh <- NewMatchedLine(NewRawLine(string(b), ecf.enableSep), nil)
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
			ecf.outCh.Send(l)
		}
	}
}
