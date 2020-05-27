package ui

func (s RangeStart) Valid() bool {
	return s.valid
}

func (s RangeStart) Value() int {
	return s.val
}

func (s *RangeStart) SetValue(n int) {
	s.val = n
	s.valid = true
}

func (s *RangeStart) Reset() {
	s.valid = false
}


