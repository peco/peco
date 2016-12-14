package line

// NewMatched creates a new Matched
func NewMatched(rl Line, matches [][]int) *Matched {
	return &Matched{rl, matches}
}

// Indices returns the indices in the buffer that matched
func (ml Matched) Indices() [][]int {
	return ml.indices
}

