package peco

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
)

type MatcherSet struct {
	current  int
	matchers []Matcher
	mutex    sync.Locker
}

func NewMatcherSet() *MatcherSet {
	return &MatcherSet{0, []Matcher{}, newMutex()}
}

func (s *MatcherSet) GetCurrent() Matcher {
	s.mutex.Lock()
	i := s.current
	s.mutex.Unlock()

	return s.Get(i)
}

func (s *MatcherSet) Get(i int) Matcher {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.matchers[i]
}

func (s *MatcherSet) SetCurrentByName(n string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, m := range s.matchers {
		if m.String() == n {
			s.current = i
			return true
		}
	}
	return false
}

func (s *MatcherSet) Add(m Matcher) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := m.Verify(); err != nil {
		return fmt.Errorf("verification for custom matcher failed: %s", err)
	}
	s.matchers = append(s.matchers, m)
	return nil
}

// Rotate rotates the matchers
func (s *MatcherSet) Rotate() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.current++
	if s.current >= len(s.matchers) {
		s.current = 0
	}
}

// Matcher interface defines the API for things that want to
// match against the buffer
type Matcher interface {
	// Match takes in three parameters.
	//
	// The first chan is the channel where cancel requests are sent.
	// If you receive a request here, you should stop running your query.
	//
	// The second is the query. Do what you want with it
	//
	// The third is the buffer in which to match the query against.
	Line(chan struct{}, string, []Line) []Line
	String() string

	// This is fugly. We just added a method only for CustomMatcner.
	// Must think about this again
	Verify() error
}

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

// RegexpMatcher is the most basic matcher
type RegexpMatcher struct {
	enableSep bool
	flags     regexpFlags
	quotemeta bool
}

// CaseSensitiveMatcher extends the RegxpMatcher, but always
// turns off the ignore-case flag in the regexp
type CaseSensitiveMatcher struct {
	*RegexpMatcher
}

// IgnoreCaseMatcher extends the RegexpMatcher, and always
// turns ON the ignore-case flag in the regexp
type IgnoreCaseMatcher struct {
	*RegexpMatcher
}

// SmartCaseMatcher turns ON the ignore-case flag in the regexp
// if the query contains a upper-case character
type SmartCaseMatcher struct {
	*RegexpMatcher
}

// CustomMatcher spawns a new process to filter the buffer
// in peco, and uses the output in its Stdout to figure
// out what to display
type CustomMatcher struct {
	enableSep bool
	name      string
	args      []string
}

// NewCaseSensitiveMatcher creates a new CaseSensitiveMatcher
func NewCaseSensitiveMatcher(enableSep bool) *CaseSensitiveMatcher {
	m := &CaseSensitiveMatcher{NewRegexpMatcher(enableSep)}
	m.quotemeta = true
	return m
}

// NewIgnoreCaseMatcher creates a new IgnoreCaseMatcher
func NewIgnoreCaseMatcher(enableSep bool) *IgnoreCaseMatcher {
	m := &IgnoreCaseMatcher{NewRegexpMatcher(enableSep)}
	m.flags = regexpFlagList(ignoreCaseFlags)
	m.quotemeta = true
	return m
}

// NewRegexpMatcher creates a new RegexpMatcher
func NewRegexpMatcher(enableSep bool) *RegexpMatcher {
	return &RegexpMatcher{
		enableSep,
		regexpFlagList(defaultFlags),
		false,
	}
}

// Verify always returns nil
func (m *RegexpMatcher) Verify() error {
	return nil
}

func containsUpper(query string) bool {
	for _, c := range query {
		if unicode.IsUpper(c) {
			return true
		}
	}
	return false
}

// NewSmartCaseMatcher creates a new SmartCaseMatcher
func NewSmartCaseMatcher(enableSep bool) *SmartCaseMatcher {
	m := &SmartCaseMatcher{NewRegexpMatcher(enableSep)}
	m.flags = regexpFlagFunc(func(q string) []string {
		if containsUpper(q) {
			return defaultFlags
		}
		return []string{"i"}
	})
	m.quotemeta = true
	return m
}

// NewCustomMatcher creates a new CustomMatcher
func NewCustomMatcher(enableSep bool, name string, args []string) *CustomMatcher {
	return &CustomMatcher{enableSep, name, args}
}

// Verify checks to see that the executable given to CustomMatcher
// is actual found and is executable via exec.LookPath
func (m *CustomMatcher) Verify() error {
	if len(m.args) == 0 {
		return fmt.Errorf("no executable specified for custom matcher '%s'", m.name)
	}

	if _, err := exec.LookPath(m.args[0]); err != nil {
		return err
	}
	return nil
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

func (m *RegexpMatcher) String() string {
	return RegexpMatch
}

func (m *CaseSensitiveMatcher) String() string {
	return CaseSensitiveMatch
}

func (m *IgnoreCaseMatcher) String() string {
	return IgnoreCaseMatch
}

func (m *SmartCaseMatcher) String() string {
	return SmartCaseMatch
}

func (m *CustomMatcher) String() string {
	return m.name
}

// sort related stuff
type byStart [][]int

func (m byStart) Len() int {
	return len(m)
}

func (m byStart) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m byStart) Less(i, j int) bool {
	if m[i][0] < m[j][0] {
		return true
	}

	if m[i][0] == m[j][0] {
		return m[i][1]-m[i][0] < m[i][1]-m[i][0]
	}

	return false
}

// Match does the heavy lifting, and matches `q` against `buffer`.
// While it is doing the match, it also listens for messages
// via `quit`. If anything is received via `quit`, the match
// is halted.
func (m *RegexpMatcher) Line(quit chan struct{}, q string, buffer []Line) []Line {
	results := []Line{}
	regexps, err := queryToRegexps(m.flags, m.quotemeta, q)
	if err != nil {
		return results
	}

	// The actual matching is done in a separate goroutine
	iter := make(chan Line, len(buffer))
	go func() {
		// This protects us from panics, caused when we cancel the
		// query and forcefully close the channel (and thereby
		// causing a "close of a closed channel"
		defer func() { recover() }()

		// This must be here to make sure the channel is properly
		// closed in normal cases
		defer close(iter)

		// Iterate through the lines, and do the match.
		// Upon success, send it through the channel
		for _, match := range buffer {
			ms := m.MatchAllRegexps(regexps, match.DisplayString())
			if ms == nil {
				continue
			}

			iter <- NewMatchedLine(match, ms)
		}
		iter <- nil
	}()

MATCH:
	for {
		select {
		case <-quit:
			// If we recieved a cancel request, we immediately bail out.
			// It's a little dirty, but we focefully terminate the other
			// goroutine by closing the channel, and invoking a panic in the
			// goroutine above

			// There's a possibility that the match fails early and the
			// cancel happens after iter has been closed. It's totally okay
			// for us to try to close iter, but trying to detect if the
			// channel can be closed safely synchronously is really hard
			// so we punt it by letting the close() happen at a separate
			// goroutine, protected by a defer recover()
			go func() {
				defer func() { recover() }()
				close(iter)
			}()
			break MATCH
		case match := <-iter:
			// Receive elements from the goroutine performing the match
			if match == nil {
				break MATCH
			}

			results = append(results, match)
		}
	}
	return results
}

func matchContains(a []int, b []int) bool {
	return a[0] <= b[0] && a[1] >= b[1]
}

func matchOverlaps(a []int, b []int) bool {
	return a[0] <= b[0] && a[1] >= b[0] ||
		a[0] <= b[1] && a[1] >= b[1]
}

func mergeMatches(a []int, b []int) []int {
	ret := make([]int, 2)

	// Note: In practice this should never happen
	// because we're sorting by N[0] before calling
	// this routine, but for completeness' sake...
	if a[0] < b[0] {
		ret[0] = a[0]
	} else {
		ret[0] = b[0]
	}

	if a[1] < b[1] {
		ret[1] = b[1]
	} else {
		ret[1] = a[1]
	}
	return ret
}

// MatchAllRegexps matches all the regexps in `regexps` against line
func (m *RegexpMatcher) MatchAllRegexps(regexps []*regexp.Regexp, line string) [][]int {
	matches := make([][]int, 0)
	allMatched := true
Line:
	for _, re := range regexps {
		match := re.FindAllStringSubmatchIndex(line, -1)
		if match == nil {
			allMatched = false
			break Line
		}

		matches = append(matches, match...)
	}

	if !allMatched {
		return nil
	}

	sort.Sort(byStart(matches))
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

	return deduped
}

// Match matches `q` aginst `buffer`
func (m *CustomMatcher) Line(quit chan struct{}, q string, buffer []Line) []Line {
	if len(m.args) < 1 {
		return []Line{}
	}

	results := []Line{}
	if q == "" {
		for _, match := range buffer {
			results = append(results, NewMatchedLine(match, nil))
		}
		return results
	}

	// Receive elements from the goroutine performing the match
	lines := []Line{}
	matcherInput := ""
	for _, match := range buffer {
		matcherInput += match.DisplayString() + "\n"
		lines = append(lines, match)
	}
	args := []string{}
	for _, arg := range m.args {
		if arg == "$QUERY" {
			arg = q
		}
		args = append(args, arg)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = strings.NewReader(matcherInput)

	// See RegexpMatcher.Match() for explanation of constructs
	iter := make(chan Line, len(buffer))
	go func() {
		defer func() { recover() }()
		defer func() {
			if p := cmd.Process; p != nil {
				p.Kill()
			}
			close(iter)
		}()
		r, err := cmd.StdoutPipe()
		if err != nil {
			iter <- nil
			return
		}
		defer r.Close()
		err = cmd.Start()
		if err != nil {
			iter <- nil
			return
		}
		buf := bufio.NewReader(r)
		for {
			b, _, err := buf.ReadLine()
			if len(b) > 0 {
				// TODO: need to redo the spec for custom matchers
				iter <- NewMatchedLine(nil, nil)
			}
			if err != nil {
				break
			}
		}
		iter <- nil
	}()
MATCH:
	for {
		select {
		case <-quit:
			go func() {
				defer func() { recover() }()
				close(iter)
			}()
			break MATCH
		case match := <-iter:
			if match == nil {
				break MATCH
			}
			results = append(results, match)
		}
	}

	return results
}
