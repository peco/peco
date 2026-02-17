package peco

import "sync"

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
