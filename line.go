package peco

import "regexp"

// Global var used to strips ansi sequences
var reANSIEscapeChars = regexp.MustCompile("\x1B\\[(?:[0-9]{1,2}(?:;[0-9]{1,2})?)*[a-zA-Z]")

// Function who strips ansi sequences
func stripANSISequence(s string) string {
	return reANSIEscapeChars.ReplaceAllString(s, "")
}

// Line defines the interface for each of the line that peco uses to display
// and match against queries. Note that to make drawing easier,
// we have a RawLine and MatchedLine types
type Line interface {
	Buffer() string        // Raw buffer, may contain null
	DisplayString() string // Line to be displayed
	Output() string        // Output string to be displayed after peco is done
	Indices() [][]int      // If the type allows, indices into matched portions of the string
}

// baseLine is the common implementation between RawLine and MatchedLine
type baseLine struct {
	buf           string
	sepLoc        int
	displayString string
}

func newBaseLine(v string, enableSep bool) *baseLine {
	m := &baseLine{
		v,
		-1,
		"",
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

func (m baseLine) Buffer() string {
	return m.buf
}

func (m baseLine) DisplayString() string {
	if m.displayString != "" {
		return m.displayString
	}

	if i := m.sepLoc; i > -1 {
		m.displayString = stripANSISequence(m.buf[:i])
	} else {
		m.displayString = stripANSISequence(m.buf)
	}
	return m.displayString
}

func (m baseLine) Output() string {
	if i := m.sepLoc; i > -1 {
		return m.buf[i+1:]
	}
	return m.buf
}

// RawLine implements the Line interface. It represents a line with no matches,
// which means that it can only be used in the initial unfiltered view
type RawLine struct {
	*baseLine
}

// NewRawLine creates a RawLine struct
func NewRawLine(v string, enableSep bool) *RawLine {
	return &RawLine{newBaseLine(v, enableSep)}
}

// Indices always returns nil
func (m RawLine) Indices() [][]int {
	return nil
}

// MatchedLine contains the actual match, and the indices to the matches
// in the line
type MatchedLine struct {
	*baseLine
	matches [][]int
}

// NewMatchedLine creates a new MatchedLine struct
func NewMatchedLine(v string, enableSep bool, m [][]int) *MatchedLine {
	return &MatchedLine{newBaseLine(v, enableSep), m}
}

// Indices returns the indices in the buffer that matched
func (d MatchedLine) Indices() [][]int {
	return d.matches
}
