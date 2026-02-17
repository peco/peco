package peco

import "sync"

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
