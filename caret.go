package peco

import "sync"

// Caret tracks the cursor position within the query line.
type Caret struct {
	mutex sync.Mutex
	pos   int
}

// Pos returns the current caret position, thread-safe.
func (c *Caret) Pos() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.pos
}

// setPosNL sets the caret position without acquiring the mutex.
// The caller must already hold the lock.
func (c *Caret) setPosNL(p int) {
	c.pos = p
}

// SetPos sets the caret position, thread-safe.
func (c *Caret) SetPos(p int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.setPosNL(p)
}

// Move moves the caret by the given delta, thread-safe.
func (c *Caret) Move(diff int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.setPosNL(c.pos + diff)
}
