package peco

import "sync"

// Caret tracks the cursor position within the query line.
type Caret struct {
	mutex sync.Mutex
	pos   int
}

func (c *Caret) Pos() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.pos
}

func (c *Caret) setPosNL(p int) {
	c.pos = p
}

func (c *Caret) SetPos(p int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.setPosNL(p)
}

func (c *Caret) Move(diff int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.setPosNL(c.pos + diff)
}
