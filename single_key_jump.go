package peco

// SingleKeyJumpState holds the state for single-key jump mode,
// where each visible line is annotated with a prefix character
// and pressing that character jumps to (and selects) the line.
type SingleKeyJumpState struct {
	mode       bool
	showPrefix bool
	prefixes   []rune
	prefixMap  map[rune]uint
}

// Mode returns whether single-key jump mode is active.
func (s *SingleKeyJumpState) Mode() bool {
	return s.mode
}

// SetMode sets single-key jump mode on or off.
func (s *SingleKeyJumpState) SetMode(b bool) {
	s.mode = b
}

// ShowPrefix returns whether jump prefixes should be displayed.
func (s *SingleKeyJumpState) ShowPrefix() bool {
	return s.showPrefix
}

// Prefixes returns the ordered list of prefix runes.
func (s *SingleKeyJumpState) Prefixes() []rune {
	return s.prefixes
}

// Index returns the line index for the given prefix rune.
func (s *SingleKeyJumpState) Index(ch rune) (uint, bool) {
	n, ok := s.prefixMap[ch]
	return n, ok
}
