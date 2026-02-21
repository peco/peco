package line

import (
	"strings"

	"github.com/google/btree"
	"github.com/peco/peco/internal/ansi"
	"github.com/peco/peco/internal/util"
)

// IDGenerator defines an interface for things that generate
// unique IDs for lines used within peco.
type IDGenerator interface {
	Next() uint64
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

	// Output returns the string to be display as peco finishes up doing its
	// thing. This means if you have null separator, the contents before the
	// separator are not included in this string
	Output() string

	// IsDirty returns true if this line should be forcefully redrawn
	IsDirty() bool

	// SetDirty sets the dirty flag on or off
	SetDirty(bool)
}

// Raw is the input line as sent to peco, before filtering and what not.
type Raw struct {
	id            uint64
	buf           string
	sepLoc        int
	displayString string
	ansiAttrs     []ansi.AttrSpan
	dirty         bool
}

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
		rl.ansiAttrs = r.Attrs
		// Only store displayString when it actually differs from buf
		// (i.e. when there's a separator or ANSI attributes)
		if rl.sepLoc > -1 || r.Attrs != nil {
			rl.displayString = r.Stripped
		}
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
		return rl.displayString
	}
	// No separator: strip ANSI (fast-path returns buf unchanged when no ESC present)
	return util.StripANSISequence(rl.buf)
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
