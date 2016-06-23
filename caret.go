package peco

func (c *Caret) Pos() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.pos
}

func (c *Caret) SetPos(p int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.pos = p
}

func (c *Caret) Move(diff int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.pos = c.pos + diff
}
