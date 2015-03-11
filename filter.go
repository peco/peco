package peco

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"unicode"
)

// These are used as keys in the config file
const (
	IgnoreCaseMatch    = "IgnoreCase"
	CaseSensitiveMatch = "CaseSensitive"
	SmartCaseMatch     = "SmartCase"
	RegexpMatch        = "Regexp"
)

var ignoreCaseFlags = []string{"i"}
var defaultFlags = []string{}

type regexpFlags interface {
	flags(string) []string
}
type regexpFlagList []string

func (r regexpFlagList) flags(_ string) []string {
	return []string(r)
}

type regexpFlagFunc func(string) []string

func (r regexpFlagFunc) flags(s string) []string {
	return r(s)
}

func containsUpper(query string) bool {
	for _, c := range query {
		if unicode.IsUpper(c) {
			return true
		}
	}
	return false
}

func regexpFor(q string, flags []string, quotemeta bool) (*regexp.Regexp, error) {
	reTxt := q
	if quotemeta {
		reTxt = regexp.QuoteMeta(q)
	}

	if flags != nil && len(flags) > 0 {
		reTxt = fmt.Sprintf("(?%s)%s", strings.Join(flags, ""), reTxt)
	}

	re, err := regexp.Compile(reTxt)
	if err != nil {
		return nil, err
	}
	return re, nil
}

func queryToRegexps(flags regexpFlags, quotemeta bool, query string) ([]*regexp.Regexp, error) {
	queries := strings.Split(strings.TrimSpace(query), " ")
	regexps := make([]*regexp.Regexp, 0)

	for _, q := range queries {
		re, err := regexpFor(q, flags.flags(query), quotemeta)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, re)
	}

	return regexps, nil
}

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
		f.rawLineBuffer.cancelCh = cancel
		f.rawLineBuffer.Replay()

		filter := f.Filter().Clone()
		filter.SetQuery(query)
		tracer.Printf("Running %#v filter using query '%s'", filter, query)

		filter.Accept(f.rawLineBuffer)
		buf := NewRawLineBuffer()
		buf.onEnd = func() { f.SendStatusMsg("") }
		buf.Accept(filter)

		f.SetActiveLineBuffer(buf)
	}

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
	String() string
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
	go acceptPipeline(cancelCh, incomingCh, rf.outputCh,
		&pipelineCtx{rf.filter, nil})
}

var ErrFilterDidNotMatch = errors.New("error: filter did not match against given line")

func (rf *RegexpFilter) filter(l Line) (Line, error) {
	tracer.Printf("RegexpFilter.filter: START")
	defer tracer.Printf("RegexpFilter.filter: END")
	regexps, err := rf.getQueryAsRegexps()
	if err != nil {
		return nil, err
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
		return nil, ErrFilterDidNotMatch
	}

	tracer.Printf("RegexpFilter.filter: line matched pattern\n")
	return NewMatchedLine(l, matches), nil
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

func (rf RegexpFilter) String() string {
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

func (fs *FilterSet) Add(qf QueryFilterer) error {
	fs.filters = append(fs.filters, qf)
	return nil
}

func (fs *FilterSet) Rotate() {
	fs.current++
	if fs.current >= len(fs.filters) {
		fs.current = 0
	}
	tracer.Printf("FilterSet.Rotate: now filter in effect is %s", fs.filters[fs.current])
}

var ErrFilterNotFound = errors.New("specified filter was not found")

func (fs *FilterSet) SetCurrentByName(name string) error {
	for i, f := range fs.filters {
		if f.String() == name {
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

// SmartCaseFilter turns ON the ignore-case flag in the regexp
// if the query contains a upper-case character
func NewSmartCaseFilter() *RegexpFilter {
	return &RegexpFilter{
		flags: regexpFlagFunc(func(q string) []string {
			if containsUpper(q) {
				return defaultFlags
			}
			return []string{"i"}
		}),
		quotemeta: true,
		name:      "SmartCase",
	}
}

type ExternalCmdFilter struct {
	simplePipeline
	cmd             string
	args            []string
	name            string
	query           string
	thresholdBufsiz int
}

func NewExternalCmdFilter(name, cmd string, args []string, threshold int) *ExternalCmdFilter {
	tracer.Printf("name = %s, cmd = %s, args = %#v", name, cmd, args)
	return &ExternalCmdFilter{
		simplePipeline:  simplePipeline{},
		cmd:             cmd,
		args:            args,
		name:            name,
		thresholdBufsiz: threshold,
	}
}

func (ecf ExternalCmdFilter) Clone() QueryFilterer {
	return &ExternalCmdFilter{
		simplePipeline:  simplePipeline{},
		cmd:             ecf.cmd,
		args:            ecf.args,
		name:            ecf.name,
		thresholdBufsiz: ecf.thresholdBufsiz,
	}
}

func (ecf *ExternalCmdFilter) Verify() error {
	if ecf.cmd == "" {
		return fmt.Errorf("no executable specified for custom matcher '%s'", ecf.name)
	}

	if _, err := exec.LookPath(ecf.cmd); err != nil {
		return err
	}
	return nil
}

func (ecf *ExternalCmdFilter) Accept(p Pipeliner) {
	cancelCh, incomingCh := p.Pipeline()
	outputCh := make(chan Line)
	ecf.cancelCh = cancelCh
	ecf.outputCh = outputCh

	go func() {
		defer close(outputCh)

		defer tracer.Printf("ExternalCmdFilter.Accept: DONE")

		// for every N lines, execute the external command
		buf := []Line{}
		for l := range incomingCh {
			buf = append(buf, l)
			if len(buf) < ecf.thresholdBufsiz {
				continue
			}

			ecf.launchExternalCmd(buf, cancelCh, outputCh)
			buf = []Line{} // drain
		}

		if len(buf) > 0 {
			ecf.launchExternalCmd(buf, cancelCh, outputCh)
		}
	}()
}

func (ecf *ExternalCmdFilter) Filter(l LineBuffer) LineBuffer { return nil }
func (ecf *ExternalCmdFilter) SetQuery(q string) {
	ecf.query = q
}

func (ecf ExternalCmdFilter) String() string {
	return ecf.name
}

func (ecf *ExternalCmdFilter) launchExternalCmd(buf []Line, cancelCh chan struct{}, outputCh chan Line) {
	defer func() { recover() }() // ignore errors

	tracer.Printf("ExternalCmdFilter.launchExternalCmd: START")
	defer tracer.Printf("ExternalCmdFilter.launchExternalCmd: END")

	tracer.Printf("buf = %v", buf)

	args := append([]string{ecf.query}, ecf.args...)
	cmd := exec.Command(ecf.cmd, args...)

	inbuf := &bytes.Buffer{}
	for _, l := range buf {
		inbuf.WriteString(l.DisplayString() + "\n")
	}

	cmd.Stdin = inbuf
	r, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	tracer.Printf("cmd = %#v", cmd)
	err = cmd.Start()
	if err != nil {
		return
	}

	go cmd.Wait()

	cmdCh := make(chan Line)
	go func(cmdCh chan Line, rdr *bufio.Reader) {
		defer tracer.Printf("Done reader")
		defer func() { recover() }()
		defer close(cmdCh)
		for {
			tracer.Printf("ReadLine")
			b, _, err := rdr.ReadLine()
			if len(b) > 0 {
				// TODO: need to redo the spec for custom matchers
				tracer.Printf("sending")
				cmdCh <- NewMatchedLine(NewRawLine(string(b), false), nil)
				tracer.Printf("sent")
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

	defer tracer.Printf("Done waiting for cancel or line")

	for {
		select {
		case <-cancelCh:
			return
		case l, ok := <-cmdCh:
			if l == nil || !ok {
				return
			}
			tracer.Printf("Custom: l = %s", l.DisplayString())
			outputCh <- l
		}
	}
}
