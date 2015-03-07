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

// RawLine implements the Line interface. It represents a line with no matches,
// which means that it can only be used in the initial unfiltered view
type RawLine  struct {
	buf           string
	sepLoc        int
	displayString string
}

func NewRawLine(v string, enableSep bool) *RawLine {
	rl := &RawLine{
		v,
		-1,
		"",
	}
	if !enableSep {
		return rl
	}

	// XXX This may be silly, but we're avoiding using strings.IndexByte()
	// here because it doesn't exist on go1.1. Let's remove support for
	// 1.1 when 1.4 comes out (or something)
	for i := 0; i < len(rl.buf); i++ {
		if rl.buf[i] == '\000' {
			rl.sepLoc = i
		}
	}
	return rl
}

func (rl RawLine) Buffer() string {
	return rl.buf
}

func (rl RawLine) DisplayString() string {
	if rl.displayString != "" {
		return rl.displayString
	}

	if i := rl.sepLoc; i > -1 {
		rl.displayString = stripANSISequence(rl.buf[:i])
	} else {
		rl.displayString = stripANSISequence(rl.buf)
	}
	return rl.displayString
}

func (rl RawLine) Output() string {
	if i := rl.sepLoc; i > -1 {
		return rl.buf[i+1:]
	}
	return rl.buf
}

// Indices always returns nil
func (m RawLine) Indices() [][]int {
	return nil
}

// MatchedLine contains the actual match, and the indices to the matches
// in the line. It also holds a reference to the orignal line
type MatchedLine struct {
	Line
	matches [][]int
}

// NewMatchedLine creates a new MatchedLine struct
func NewMatchedLine(l Line, m [][]int) *MatchedLine {
	return &MatchedLine{
		Line: l,
		matches: m,
	}
}

// Indices returns the indices in the buffer that matched
func (d MatchedLine) Indices() [][]int {
	return d.matches
}
