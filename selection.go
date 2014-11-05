package peco

import (
	"math/big"
	"sync"
)

// Selection stores the line numbers that were selected by the user.
// The contents of the Selection is always sorted from smallest to
// largest line number
type Selection struct {
	selection *big.Int // bitmask
	flipped   uint64
	mutex     sync.Locker
}

func NewSelection() *Selection {
	return &Selection{&big.Int{}, 0, newMutex()}
}

func (s *Selection) Invert(pad int) {
	dst := (&big.Int{}).Set(s.selection)
	for i := 0; i < pad; i++ {
		b := dst.Bit(i)
		if b == 1 {
			b = 0
		} else {
			b = 1
		}
		dst.SetBit(dst, i, b)
		if b == 1 {
			s.flipped++
		}
	}

	s.selection = dst
}

// Has returns true if line `v` is in the selection
func (s Selection) Has(v int) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.selection.Bit(v) == 1
}

// Add adds a new line number to the selection. If the line already
// exists in the selection, it is silently ignored
func (s *Selection) Add(v int) {
	if s.Has(v) {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.flipped++
	s.selection = s.selection.SetBit(s.selection, v, 1)
}

// Remove removes the specified line number from the selection
func (s *Selection) Remove(v int) {
	if ! s.Has(v) {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.flipped--
	s.selection = s.selection.SetBit(s.selection, v, 0)
}

// Clear empties the selection
func (s *Selection) Clear() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.flipped = 0
	s.selection = &big.Int{}
}

// Len returns the number of elements in the selection.
func (s Selection) Len() uint64 {
	return s.flipped
}
