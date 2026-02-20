package peco

import (
	"sync"

	"github.com/google/btree"
	"github.com/peco/peco/line"
)

// Selection stores the line ids that were selected by the user.
// The contents of the Selection is always sorted from smallest to
// largest line ID
type Selection struct {
	mutex sync.RWMutex
	tree  *btree.BTree
}

// RangeStart tracks the starting position of a range selection.
type RangeStart struct {
	val   int
	valid bool
}

// NewSelection creates a new empty Selection
func NewSelection() *Selection {
	s := &Selection{}
	s.Reset()
	return s
}

// Add adds a new line to the selection. If the line already
// exists in the selection, it is silently ignored
func (s *Selection) Add(l line.Line) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.tree.ReplaceOrInsert(l)
}

// Copy copies all selected lines from s into dst.
func (s *Selection) Copy(dst *Selection) {
	s.Ascend(func(it btree.Item) bool {
		l, ok := it.(line.Line)
		if !ok {
			return true
		}
		dst.Add(l)
		return true
	})
}

// Remove removes the specified line from the selection
func (s *Selection) Remove(l line.Line) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.tree.Delete(l)
}

// Reset clears all selected indices from the selection.
func (s *Selection) Reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.tree = btree.New(32)
}

// Has reports whether the given line is in the selection.
func (s *Selection) Has(x line.Line) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.tree.Has(x)
}

// Len returns the number of selected lines.
func (s *Selection) Len() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.tree.Len()
}

// Ascend iterates over selected lines in ascending order, calling i for each.
func (s *Selection) Ascend(i btree.ItemIterator) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	s.tree.Ascend(i)
}
