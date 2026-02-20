package selection

import (
	"sync"

	"github.com/google/btree"
	"github.com/peco/peco/line"
)

// Set stores the line ids that were selected by the user.
// The contents of the Set is always sorted from smallest to
// largest line ID.
type Set struct {
	mutex sync.RWMutex
	tree  *btree.BTree
}

// RangeStart tracks the starting position of a range selection.
type RangeStart struct {
	val   int
	valid bool
}

// New creates a new empty Set.
func New() *Set {
	s := &Set{}
	s.Reset()
	return s
}

// Add adds a new line to the selection. If the line already
// exists in the selection, it is silently ignored.
func (s *Set) Add(l line.Line) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.tree.ReplaceOrInsert(l)
}

// Copy copies all selected lines from s into dst.
//
// Items are collected under the source read lock first, then added to dst
// after releasing that lock. This avoids deadlocks when dst == s and
// prevents ABBA deadlocks during concurrent bidirectional copies.
func (s *Set) Copy(dst *Set) {
	s.mutex.RLock()
	items := make([]line.Line, 0, s.tree.Len())
	s.tree.Ascend(func(it btree.Item) bool {
		if l, ok := it.(line.Line); ok {
			items = append(items, l)
		}
		return true
	})
	s.mutex.RUnlock()

	for _, l := range items {
		dst.Add(l)
	}
}

// Remove removes the specified line from the selection.
func (s *Set) Remove(l line.Line) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.tree.Delete(l)
}

// Reset clears all selected indices from the selection.
func (s *Set) Reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.tree = btree.New(32)
}

// Has reports whether the given line is in the selection.
func (s *Set) Has(x line.Line) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.tree.Has(x)
}

// Len returns the number of selected lines.
func (s *Set) Len() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.tree.Len()
}

// Ascend iterates over selected lines in ascending order, calling fn for each.
func (s *Set) Ascend(fn func(line.Line) bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	s.tree.Ascend(func(it btree.Item) bool {
		l, ok := it.(line.Line)
		if !ok {
			return true
		}
		return fn(l)
	})
}

// Valid reports whether the RangeStart has been set.
func (s RangeStart) Valid() bool {
	return s.valid
}

// Value returns the starting line index of the range.
func (s RangeStart) Value() int {
	return s.val
}

// SetValue sets the starting position and marks it as valid.
func (s *RangeStart) SetValue(n int) {
	s.val = n
	s.valid = true
}

// Reset clears the range start, marking it as invalid.
func (s *RangeStart) Reset() {
	s.valid = false
}
