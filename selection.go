package peco

import "github.com/google/btree"

// Selection stores the line ids that were selected by the user.
// The contents of the Selection is always sorted from smallest to
// largest line ID
type Selection struct{ *btree.BTree }

// NewSelection creates a new empty Selection
func NewSelection() *Selection {
	return &Selection{btree.New(32)}
}

// Add adds a new line to the selection. If the line already
// exists in the selection, it is silently ignored
func (s *Selection) Add(l Line) {
	s.ReplaceOrInsert(l)
}

// Remove removes the specified line from the selection
func (s *Selection) Remove(l Line) {
	s.Delete(l)
}
