package peco

import "sort"

// Selection stores the line numbers that were selected by the user.
// The contents of the Selection is always sorted from smallest to
// largest line number
type Selection []int

// Has returns true if line `v` is in the selection
func (s Selection) Has(v int) bool {
	for _, i := range []int(s) {
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
	*s = Selection(append([]int(*s), v))
	sort.Sort(s)
}

// Remove removes the specified line number from the selection
func (s *Selection) Remove(v int) {
	a := []int(*s)
	for k, i := range a {
		if i == v {
			tmp := a[:k]
			tmp = append(tmp, a[k+1:]...)
			*s = Selection(tmp)
			return
		}
	}
}

// Clear empties the selection
func (s *Selection) Clear() {
	*s = Selection([]int{})
}

// Len returns the number of elements in the selection. Satisfies
// sort.Interface
func (s Selection) Len() int {
	return len(s)
}

// Swap swaps the elements in indices i and j. Satisfies sort.Interface
func (s Selection) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns true if element at index i is less than the element at
// index j. Satisfies sort.Interface
func (s Selection) Less(i, j int) bool {
	return s[i] < s[j]
}
