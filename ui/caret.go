package ui

func (c *Caret) Pos() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.pos
}

func (c *Caret) setPos_nolock(p int) {
	c.pos = p
}

func (c *Caret) SetPos(p int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.setPos_nolock(p)
}

func (c *Caret) Move(diff int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.setPos_nolock(c.pos + diff)
}
