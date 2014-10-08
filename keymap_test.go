package peco

import "testing"

func TestSelection(t *testing.T) {
	s := NewSelection()

	s.Add(10)
	if s.Len() != 1 {
		t.Errorf("expected Len = 1, got %d", s.Len())
	}
	s.Add(1)
	if s.Len() != 2 {
		t.Errorf("expected Len = 2, got %d", s.Len())
	}
	s.Add(1)
	if s.Len() != 2 {
		t.Errorf("expected Len = 2, got %d", s.Len())
	}
	s.Remove(1)
	if s.Len() != 1 {
		t.Errorf("expected Len = 1, got %d", s.Len())
	}
}
