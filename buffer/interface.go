package buffer

import (
	"sync"

	"github.com/peco/peco/line"
)

// Buffer interface is used for containers for lines to be
// processed by peco.
type Buffer interface {
	LinesInRange(int, int) []line.Line
	LineAt(int) (line.Line, error)
	Size() int
}



// Filtered holds a "filtered" buffer. It holds a reference to
// the source buffer (note: should be immutable) and a list of indices
// into the source buffer
type Filtered struct {
	maxcols   int
	src       Buffer
	selection []int // maps from our index to src's index
}

// Memory is an implementation of Buffer
type Memory struct {
	done         chan struct{}
	lines        []line.Line
	mutex        sync.RWMutex
	PeriodicFunc func()
}
