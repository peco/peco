package line

import "github.com/google/btree"

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
	dirty         bool
}

// Matched contains the indices to the matches
type Matched struct {
	Line
	indices [][]int
}


