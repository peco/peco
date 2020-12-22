package peco

import (
	"sync"
)

type PrintCmd interface {
	X(int) PrintCmd
	Y(int) PrintCmd
	XOffset(int) PrintCmd
	Style(*Style) PrintCmd
	Fill(bool) PrintCmd
	Do() int
}

// printCmd is meant to provide the basic functionality common to
// all underlying output (terminal) types. It is meant to be wrapped
// by another struct, which delegates calls to it.
type printCmd struct {
	x       int
	xOffset int
	y       int
	style   *Style
	msg     string
	fill    bool
}

var printCmdPool = sync.Pool{
	New: func() interface{} { return &printCmd{} },
}

func (cmd *printCmd) Release() {
	cmd.x = 0
	cmd.xOffset = 0
	cmd.y = 0
	cmd.style = nil
	cmd.msg = ""
	cmd.fill = false
	printCmdPool.Put(cmd)
}

func (cmd *printCmd) X(v int) {
	cmd.x = v
}

func (cmd *printCmd) XOffset(v int) {
	cmd.xOffset = v
}

func (cmd *printCmd) Y(v int) {
	cmd.y = v
}

func (cmd *printCmd) Style(s *Style) {
	cmd.style = s
}

func (cmd *printCmd) Fill(v bool) {
	cmd.fill = v
}
