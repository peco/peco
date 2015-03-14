package peco

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/nsf/termbox-go"
)

func parseAttrib(currfg, currbg termbox.Attribute, esc string) (fg, bg termbox.Attribute) {
	fg, bg = currfg, currbg

	codes := strings.Split(esc, ";")
	for _, code := range codes {
		switch code {
		case "30":
			fg &= 16 // zero color bits
			fg |= termbox.ColorBlack
		case "31":
			fg &= 16 // zero color bits
			fg |= termbox.ColorRed
		case "32":
			fg &= 16 // zero color bits
			fg |= termbox.ColorGreen
		case "33":
			fg &= 16 // zero color bits
			fg |= termbox.ColorYellow
		case "34":
			fg &= 16 // zero color bits
			fg |= termbox.ColorBlue
		case "35":
			fg &= 16 // zero color bits
			fg |= termbox.ColorMagenta
		case "36":
			fg &= 16 // zero color bits
			fg |= termbox.ColorCyan
		case "37":
			fg &= 16 // zero color bits
			fg |= termbox.ColorWhite
		case "39":
			fg &= 16 // zero color bits
			fg |= termbox.ColorDefault
		case "40":
			bg &= 16 // zero color bits
			bg |= termbox.ColorBlack
		case "41":
			bg &= 16 // zero color bits
			bg |= termbox.ColorRed
		case "42":
			bg &= 16 // zero color bits
			bg |= termbox.ColorGreen
		case "43":
			bg &= 16 // zero color bits
			bg |= termbox.ColorYellow
		case "44":
			bg &= 16 // zero color bits
			bg |= termbox.ColorBlue
		case "45":
			bg &= 16 // zero color bits
			bg |= termbox.ColorMagenta
		case "46":
			bg &= 16 // zero color bits
			bg |= termbox.ColorCyan
		case "47":
			bg &= 16 // zero color bits
			bg |= termbox.ColorWhite
		case "49":
			bg &= 16 // zero color bits
			bg |= termbox.ColorDefault
		case "0", "": // reset all attribs
			bg = termbox.ColorDefault
			fg = termbox.ColorDefault
		}
	}

	return fg, bg
}

// Match defines the interface for matches. Note that to make drawing easier,
// we have a DidMatch and NoMatch types instead of using []Match and []string.
type Match interface {
	Buffer() string // Raw buffer, may contain null
	Line() string   // Line to be displayed
	Output() string // Output string to be displayed after peco is done
	Attribs() (fg, bg []termbox.Attribute)
	Indices() [][]int
}

type MatchString struct {
	buf    string
	lbuf   string // buf with escape sequences removed for Line() output
	sepLoc int
	bg     []termbox.Attribute
	fg     []termbox.Attribute
}

func NewMatchString(v string, enableSep bool) *MatchString {
	m := &MatchString{
		v,
		v,
		-1,
		nil,
		nil,
	}

	// TODO: handle sepLoc correctly assuming lbuf != buf
	m.bg = make([]termbox.Attribute, len(m.lbuf))
	m.fg = make([]termbox.Attribute, len(m.lbuf))
	for {
		start := strings.Index(m.lbuf, string(0x1b)+"[")
		if start == -1 {
			break
		}
		end := strings.Index(m.lbuf[start:], "m")
		if end == -1 {
			panic("unterminated escape sequence") // TODO: how to handle this?
		}
		end += start + 1
		for i := end; i < len(m.lbuf); i++ {
			m.fg[i], m.bg[i] = parseAttrib(m.fg[i], m.bg[i], m.lbuf[start+2:end-1])
		}
		m.lbuf = m.lbuf[:start] + m.lbuf[end:]
		m.fg = append(append([]termbox.Attribute{}, m.fg[:start]...), m.fg[end:]...)
		m.bg = append(append([]termbox.Attribute{}, m.bg[:start]...), m.bg[end:]...)
	}

	if !enableSep {
		return m
	}

	// XXX This may be silly, but we're avoiding using strings.IndexByte()
	// here because it doesn't exist on go1.1. Let's remove support for
	// 1.1 when 1.4 comes out (or something)
	for i := 0; i < len(m.buf); i++ {
		if m.buf[i] == '\000' {
			m.sepLoc = i
		}
	}
	return m
}

func (m MatchString) Attribs() (fgs, bgs []termbox.Attribute) {
	return m.fg, m.bg
}

func (m MatchString) Buffer() string {
	return m.buf
}

func (m MatchString) Line() string {
	if i := m.sepLoc; i > -1 {
		return m.lbuf[:i]
	}
	return m.lbuf
}

func (m MatchString) Output() string {
	if i := m.sepLoc; i > -1 {
		return m.buf[i+1:]
	}
	return m.buf
}

// NoMatch is actually an alias to a regular string. It implements the
// Match interface, but just returns the underlying string with no matches
type NoMatch struct {
	*MatchString
}

func NewNoMatch(v string, enableSep bool) *NoMatch {
	return &NoMatch{NewMatchString(v, enableSep)}
}

func (m NoMatch) Indices() [][]int {
	return nil
}

// DidMatch contains the actual match, and the indices to the matches
// in the line
type DidMatch struct {
	*MatchString
	matches [][]int
}

func NewDidMatch(v string, enableSep bool, m [][]int) *DidMatch {
	return &DidMatch{NewMatchString(v, enableSep), m}
}

func (d DidMatch) Indices() [][]int {
	return d.matches
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
	Match(chan struct{}, string, []Match) []Match
	String() string
}

const (
	IgnoreCaseMatch    = "IgnoreCase"
	CaseSensitiveMatch = "CaseSensitive"
	RegexpMatch        = "Regexp"
)

type RegexpMatcher struct {
	enableSep bool
	flags     []string
	quotemeta bool
}

type CaseSensitiveMatcher struct {
	*RegexpMatcher
}

type IgnoreCaseMatcher struct {
	*RegexpMatcher
}

type CustomMatcher struct {
	enableSep bool
	name      string
	args      []string
}

func NewCaseSensitiveMatcher(enableSep bool) *CaseSensitiveMatcher {
	m := &CaseSensitiveMatcher{NewRegexpMatcher(enableSep)}
	m.quotemeta = true
	return m
}

func NewIgnoreCaseMatcher(enableSep bool) *IgnoreCaseMatcher {
	m := &IgnoreCaseMatcher{NewRegexpMatcher(enableSep)}
	m.flags = []string{"i"}
	m.quotemeta = true
	return m
}

func NewRegexpMatcher(enableSep bool) *RegexpMatcher {
	return &RegexpMatcher{
		enableSep,
		[]string{},
		false,
	}
}

func NewCustomMatcher(enableSep bool, name string, args []string) *CustomMatcher {
	return &CustomMatcher{enableSep, name, args}
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

func (m *RegexpMatcher) QueryToRegexps(query string) ([]*regexp.Regexp, error) {
	queries := strings.Split(strings.TrimSpace(query), " ")
	regexps := make([]*regexp.Regexp, 0)

	for _, q := range queries {
		re, err := regexpFor(q, m.flags, m.quotemeta)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, re)
	}

	return regexps, nil
}

func (m *RegexpMatcher) String() string {
	return "Regexp"
}

func (m *CaseSensitiveMatcher) String() string {
	return "CaseSensitive"
}

func (m *IgnoreCaseMatcher) String() string {
	return "IgnoreCase"
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
	return m[i][0] < m[j][0]
}

func (m *RegexpMatcher) Match(quit chan struct{}, q string, buffer []Match) []Match {
	results := []Match{}
	regexps, err := m.QueryToRegexps(q)
	if err != nil {
		return results
	}

	// The actual matching is done in a separate goroutine
	iter := make(chan Match, len(buffer))
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
			ms := m.MatchAllRegexps(regexps, match.Line())
			if ms == nil {
				continue
			}
			iter <- NewDidMatch(match.Buffer(), m.enableSep, ms)
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

func (m *RegexpMatcher) MatchAllRegexps(regexps []*regexp.Regexp, line string) [][]int {
	matches := make([][]int, 0)

	allMatched := true
Match:
	for _, re := range regexps {
		match := re.FindAllStringSubmatchIndex(line, -1)
		if match == nil {
			allMatched = false
			break Match
		}

		for _, ma := range match {
			start, end := ma[0], ma[1]
			for _, m := range matches {
				if start >= m[0] && start < m[1] {
					continue Match
				}

				if start < m[0] && end >= m[0] {
					continue Match
				}
			}
			matches = append(matches, ma)
		}
	}

	if !allMatched {
		return nil
	}

	sort.Sort(byStart(matches))

	return matches
}

func (m *CustomMatcher) Match(quit chan struct{}, q string, buffer []Match) []Match {
	if len(m.args) < 1 {
		return []Match{}
	}

	results := []Match{}
	if q == "" {
		for _, match := range buffer {
			results = append(results, NewDidMatch(match.Buffer(), m.enableSep, nil))
		}
		return results
	}

	// Receive elements from the goroutine performing the match
	lines := []Match{}
	matcherInput := ""
	for _, match := range buffer {
		matcherInput += match.Line() + "\n"
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
	iter := make(chan Match, len(buffer))
	go func() {
		defer func() { recover() }()
		defer func() {
			if p := cmd.Process; p != nil {
				p.Kill()
			}
			close(iter)
		}()
		b, err := cmd.Output()
		if err != nil {
			iter <- nil
		}
		for _, line := range strings.Split(string(b), "\n") {
			if len(line) > 0 {
				iter <- NewDidMatch(line, m.enableSep, nil)
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
