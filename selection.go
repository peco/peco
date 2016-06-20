package peco

import "github.com/google/btree"

// NewSelection creates a new empty Selection
func NewSelection() *Selection {
	s := &Selection{}
	s.Reset()
	return s
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

func (s *Selection) Reset() {
	s.BTree = btree.New(32)
}
