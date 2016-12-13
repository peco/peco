package peco

import (
	"strings"

	"github.com/google/btree"
	"github.com/peco/peco/internal/util"
)

// NewRawLine creates a new RawLine. The `enableSep` flag tells
// it if we should search for a null character to split the
// string to display and the string to emit upon selection of
// of said line
func NewRawLine(id uint64, v string, enableSep bool) *RawLine {
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
		rl.displayString = util.StripANSISequence(rl.buf[:i])
	} else {
		rl.displayString = util.StripANSISequence(rl.buf)
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

// NewMatchedLine creates a new MatchedLine
func NewMatchedLine(rl Line, matches [][]int) *MatchedLine {
	return &MatchedLine{rl, matches}
}

// Indices returns the indices in the buffer that matched
func (ml MatchedLine) Indices() [][]int {
	return ml.indices
}
