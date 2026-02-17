package line

import (
	"strings"

	"github.com/google/btree"
	"github.com/peco/peco/internal/ansi"
	"github.com/peco/peco/internal/util"
)

// NewRaw creates a new Raw. The `enableSep` flag tells
// it if we should search for a null character to split the
// string to display and the string to emit upon selection of
// said line. The `enableANSI` flag enables ANSI SGR parsing.
func NewRaw(id uint64, v string, enableSep bool, enableANSI bool) *Raw {
	rl := &Raw{
		id:            id,
		buf:           v,
		sepLoc:        -1,
		displayString: "",
		dirty:         false,
	}

	if enableSep {
		if i := strings.IndexByte(rl.buf, '\000'); i != -1 {
			rl.sepLoc = i
		}
	}

	if enableANSI {
		// Determine which portion to parse for display
		src := rl.buf
		if rl.sepLoc > -1 {
			src = rl.buf[:rl.sepLoc]
		}
		r := ansi.Parse(src)
		rl.displayString = r.Stripped
		rl.ansiAttrs = r.Attrs
	}

	return rl
}

// Less implements the btree.Item interface
func (rl *Raw) Less(b btree.Item) bool {
	l, ok := b.(Line)
	if !ok {
		return false
	}
	return rl.id < l.ID()
}

// ID returns the unique ID of this line
func (rl *Raw) ID() uint64 {
	return rl.id
}

// IsDirty returns true if this line must be redrawn on the terminal
func (rl *Raw) IsDirty() bool {
	return rl.dirty
}

// SetDirty sets the dirty flag
func (rl *Raw) SetDirty(b bool) {
	rl.dirty = b
}

// Buffer returns the raw buffer. May contain null
func (rl *Raw) Buffer() string {
	return rl.buf
}

// DisplayString returns the string to be displayed
func (rl *Raw) DisplayString() string {
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

// ANSIAttrs returns the run-length encoded ANSI attributes for this line.
// Returns nil if ANSI parsing was not enabled or the line had no ANSI codes.
func (rl *Raw) ANSIAttrs() []ansi.AttrSpan {
	return rl.ansiAttrs
}

// Output returns the string to be displayed *after peco is done
func (rl *Raw) Output() string {
	if i := rl.sepLoc; i > -1 {
		return rl.buf[i+1:]
	}
	return rl.buf
}
