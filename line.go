package peco

import "regexp"

// Global var used to strips ansi sequences
var reANSIEscapeChars = regexp.MustCompile("\x1B\\[(?:[0-9]{1,2}(?:;[0-9]{1,2})?)*[a-zA-Z]")

// Function who strips ansi sequences
func stripANSISequence(s string) string {
	return reANSIEscapeChars.ReplaceAllString(s, "")
}

// Line represents each of the line that peco uses to display
// and match against queries. 
type Line interface {
	Buffer() string
	DisplayString() string
	Indices() [][]int
	Output() string
	IsDirty() bool
	SetDirty(bool)
}

type RawLine struct {
	buf           string
	sepLoc        int
	displayString string
	dirty         bool
}

func NewRawLine(v string, enableSep bool) *RawLine {
	rl := &RawLine{
		v,
		-1,
		"",
		false,
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

func (rl RawLine) IsDirty() bool {
	return rl.dirty
}

func (rl *RawLine) SetDirty(b bool) {
	rl.dirty = b
}

// Buffer returns the raw buffer. May contain null
func (rl RawLine) Buffer() string {
	return rl.buf
}

// DisplayString returns the string to be displayed
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


// Output returns the string to be displayed *after peco is done
func (rl RawLine) Output() string {
	if i := rl.sepLoc; i > -1 {
		return rl.buf[i+1:]
	}
	return rl.buf
}

func (rl RawLine) Indices() [][]int {
	return nil
}

type MatchedLine struct {
	Line
	indices [][]int
}

func NewMatchedLine(rl Line, matches [][]int) *MatchedLine {
	return &MatchedLine{rl, matches}
}

// Indices returns the indices in the buffer that matched
func (ml MatchedLine) Indices() [][]int {
	return ml.indices
}
