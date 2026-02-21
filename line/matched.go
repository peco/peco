package line

import (
	"sync"

	"github.com/peco/peco/internal/ansi"
)

var matchedPool = sync.Pool{
	New: func() any { return &Matched{} },
}

// GetMatched retrieves a Matched from the pool and initializes it.
func GetMatched(rl Line, matches [][]int) *Matched {
	m := matchedPool.Get().(*Matched)
	m.Line = rl
	m.indices = matches
	return m
}

// ReleaseMatched returns a Matched to the pool after clearing its fields.
func ReleaseMatched(m *Matched) {
	m.Line = nil
	m.indices = nil
	matchedPool.Put(m)
}

// Matched contains the indices to the matches
type Matched struct {
	Line
	indices [][]int
}

// NewMatched creates a new Matched
func NewMatched(rl Line, matches [][]int) *Matched {
	return &Matched{rl, matches}
}

// Indices returns the indices in the buffer that matched
func (ml Matched) Indices() [][]int {
	return ml.indices
}

// ANSIAttrs returns the ANSI attributes from the underlying line, if available.
func (ml Matched) ANSIAttrs() []ansi.AttrSpan {
	if r, ok := ml.Line.(*Raw); ok {
		return r.ANSIAttrs()
	}
	return nil
}
