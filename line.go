package peco

import (
	"regexp"
	"strings"

	"github.com/google/btree"
)

type idGen struct {
	genCh chan uint64
}

func newIDGen() *idGen {
	ch := make(chan uint64)
	go func() {
		var i uint64
		for ; ; i++ {
			ch <- i
			if i >= uint64(1<<63)-1 {
				i = 0
			}
		}
	}()
	return &idGen{
		genCh: ch,
	}
}

func (ig *idGen) create() uint64 {
	return <-ig.genCh
}

// Global var used to strips ansi sequences
var reANSIEscapeChars = regexp.MustCompile("\x1B\\[(?:[0-9]{1,2}(?:;[0-9]{1,2})?)*[a-zA-Z]")

// Function who strips ansi sequences
func stripANSISequence(s string) string {
	return reANSIEscapeChars.ReplaceAllString(s, "")
}

// Line represents each of the line that peco uses to display
// and match against queries.
type Line interface {
	btree.Item

	ID() uint64

	// Buffer returns the raw buffer
	Buffer() string

	// DisplayString returns the string to be displayed. This means if you have
	// a null separator, the contents after the separator are not included
	// in this string
	DisplayString() string

	// Indices return the matched portion(s) of a string after filtering.
	// Note that while Indices may return nil, that just means that there are
	// no substrings to be hilighted. It doesn't mean there were no matches
	Indices() [][]int

	// Output returns the string to be display as peco finishes up doing its
	// thing. This means if you have null separator, the contents before the
	// separater are not included in this string
	Output() string

	// IsDirty returns true if this line should be forcefully redrawn
	IsDirty() bool

	// SetDirty sets the dirty flag on or off
	SetDirty(bool)
}

// RawLine is the input line as sent to peco, before filtering and what not.
type RawLine struct {
	id            uint64
	buf           string
	sepLoc        int
	displayString string
	dirty         bool
}

var idGenerator = newIDGen()

// NewRawLine creates a new RawLine. The `enableSep` flag tells
// it if we should search for a null character to split the
// string to display and the string to emit upon selection of
// of said line
func NewRawLine(v string, enableSep bool) *RawLine {
	id := idGenerator.create()
	rl := &RawLine{
		id:            id,
		buf:           v,
		sepLoc:        -1,
		displayString: "",
		dirty:         false,
	}

	if !enableSep {
		return rl
	}

	if i := strings.IndexByte(rl.buf, '\000'); i != -1 {
		rl.sepLoc = i
	}
	return rl
}

// Less implements the btree.Item interface
func (rl *RawLine) Less(b btree.Item) bool {
	return rl.id < b.(Line).ID()
}

// ID returns the unique ID of this line
func (rl *RawLine) ID() uint64 {
	return rl.id
}

// IsDirty returns true if this line must be redrawn on the terminal
func (rl RawLine) IsDirty() bool {
	return rl.dirty
}

// SetDirty sets the dirty flag
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

// Indices fulfills the Line interface, but for RawLine it always
// returns nil
func (rl RawLine) Indices() [][]int {
	return nil
}

// MatchedLine contains the indices to the matches
type MatchedLine struct {
	Line
	indices [][]int
}

// NewMatchedLine creates a new MatchedLine
func NewMatchedLine(rl Line, matches [][]int) *MatchedLine {
	return &MatchedLine{rl, matches}
}

// Indices returns the indices in the buffer that matched
func (ml MatchedLine) Indices() [][]int {
	return ml.indices
}
