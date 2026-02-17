package peco

import (
	"sync"
	"time"
)

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

// ZoomState holds the saved buffer and cursor position from before
// a ZoomIn operation, so that ZoomOut can restore them. A nil buffer
// means not currently zoomed.
type ZoomState struct {
	mutex  sync.Mutex
	buffer Buffer
	lineNo int
}

// Buffer returns the saved buffer from before ZoomIn, or nil if not zoomed.
func (z *ZoomState) Buffer() Buffer {
	z.mutex.Lock()
	defer z.mutex.Unlock()
	return z.buffer
}

// LineNo returns the saved cursor position from before ZoomIn.
func (z *ZoomState) LineNo() int {
	z.mutex.Lock()
	defer z.mutex.Unlock()
	return z.lineNo
}

// Set saves the current buffer and cursor position before zooming in.
func (z *ZoomState) Set(buf Buffer, lineNo int) {
	z.mutex.Lock()
	defer z.mutex.Unlock()
	z.buffer = buf
	z.lineNo = lineNo
}

// Clear clears the saved zoom state.
func (z *ZoomState) Clear() {
	z.mutex.Lock()
	defer z.mutex.Unlock()
	z.buffer = nil
	z.lineNo = 0
}

// FrozenState holds a snapshot of filter results when the user
// "freezes" the current results to filter on top of them.
type FrozenState struct {
	mutex  sync.Mutex
	source *MemoryBuffer
}

// Source returns the frozen source buffer, or nil if not frozen.
func (f *FrozenState) Source() *MemoryBuffer {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.source
}

// Set saves the given buffer as the frozen source.
func (f *FrozenState) Set(buf *MemoryBuffer) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.source = buf
}

// Clear clears the frozen source.
func (f *FrozenState) Clear() {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.source = nil
}

// QueryExecState holds the state for delayed query execution,
// including the delay duration, a mutex guarding the timer,
// and the timer itself.
type QueryExecState struct {
	delay time.Duration
	mutex sync.Mutex
	timer *time.Timer
}

// Delay returns the query execution delay.
func (q *QueryExecState) Delay() time.Duration {
	return q.delay
}

// StopTimer stops and clears the pending query exec timer.
// It must be called during shutdown to prevent the timer callback
// from firing after program state is torn down.
func (q *QueryExecState) StopTimer() {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.timer != nil {
		q.timer.Stop()
		q.timer = nil
	}
}
