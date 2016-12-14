package peco

import (
	"testing"

	"github.com/peco/peco/line"
)

func TestSelection(t *testing.T) {
	s := NewSelection()

	var i uint64 = 0
	alice := line.NewRaw(i, "Alice", false)
	i++
	s.Add(alice)
	if s.Len() != 1 {
		t.Errorf("expected Len = 1, got %d", s.Len())
	}
	s.Add(line.NewRaw(i, "Bob", false))
	i++
	if s.Len() != 2 {
		t.Errorf("expected Len = 2, got %d", s.Len())
	}
	s.Add(alice)
	if s.Len() != 2 {
		t.Errorf("expected Len = 2, got %d", s.Len())
	}
	s.Remove(alice)
	if s.Len() != 1 {
		t.Errorf("expected Len = 1, got %d", s.Len())
	}
}
