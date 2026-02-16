package line

import "github.com/peco/peco/internal/ansi"

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
