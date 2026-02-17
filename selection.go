package peco

import (
	"github.com/google/btree"
	"github.com/peco/peco/line"
)

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

func (s *Selection) Reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.tree = btree.New(32)
}

func (s *Selection) Has(x line.Line) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.tree.Has(x)
}

func (s *Selection) Len() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.tree.Len()
}

func (s *Selection) Ascend(i btree.ItemIterator) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	s.tree.Ascend(i)
}
