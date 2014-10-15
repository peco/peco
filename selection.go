package peco

import (
	"sort"
	"sync"
)

// Selection stores the line numbers that were selected by the user.
// The contents of the Selection is always sorted from smallest to
// largest line number
type Selection struct {
	selection []int
	mutex     sync.Locker
}

func NewSelection() *Selection {
	return &Selection{nil, newMutex()}
}

func (s *Selection) GetSelection() []int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.selection[:]
}

// Has returns true if line `v` is in the selection
func (s Selection) Has(v int) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, i := range s.selection {
		if i == v {
			return true
		}
	}
	return false
}

// Add adds a new line number to the selection. If the line already
// exists in the selection, it is silently ignored
func (s *Selection) Add(v int) {
	if s.Has(v) {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.selection = append(s.selection, v)
	sort.Sort(s)
}

// Remove removes the specified line number from the selection
func (s *Selection) Remove(v int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for k, i := range s.selection {
		if i == v {
			tmp := s.selection[:k]
			tmp = append(tmp, s.selection[k+1:]...)
			s.selection = tmp
			return
		}
	}
}

// Clear empties the selection
func (s *Selection) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.selection = []int{}
}

// Len returns the number of elements in the selection. Satisfies
// sort.Interface
func (s Selection) Len() int {
	return len(s.selection)
}

// Swap swaps the elements in indices i and j. Satisfies sort.Interface
func (s *Selection) Swap(i, j int) {
	s.selection[i], s.selection[j] = s.selection[j], s.selection[i]
}

// Less returns true if element at index i is less than the element at
// index j. Satisfies sort.Interface
func (s Selection) Less(i, j int) bool {
	return s.selection[i] < s.selection[j]
}
