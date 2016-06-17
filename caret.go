package peco

type Caret struct {
	pos int
}

func (c Caret) Pos() int {
	return c.pos
}

func (c *Caret) SetPos(p int) {
	c.pos = p
}

func (c *Caret) Move(diff int) {
	c.SetPos(c.Pos() + diff)
}
