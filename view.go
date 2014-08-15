package peco

import (
	"time"

	"github.com/nsf/termbox-go"
)

// View handles the drawing/updating the screen
type View struct {
	*Ctx
	layout Layout
}

// PagingRequest can be sent to move the selection cursor
type PagingRequest int

const (
	// ToNextLine moves the selection to the next line
	ToNextLine PagingRequest = iota
	// ToNextPage moves the selection to the next page
	ToNextPage
	// ToPrevLine moves the selection to the previous line
	ToPrevLine
	// ToPrevPage moves the selection to the previous page
	ToPrevPage
)

// Loop receives requests to update the screen
func (v *View) Loop() {
	defer v.ReleaseWaitGroup()
	for {
		select {
		case <-v.LoopCh():
			return
		case m := <-v.StatusMsgCh():
			v.printStatus(m.DataString())
			m.Done()
		case m := <-v.ClearStatusCh():
			v.clearStatus(m.DataInterface().(time.Duration))
			m.Done()
		case r := <-v.PagingCh():
			v.movePage(r.DataInterface().(PagingRequest))
			r.Done()
		case lines := <-v.DrawCh():
			v.drawScreen(lines.DataInterface().([]Match))
			lines.Done()
		}
	}
}

func (v *View) printStatus(m string) {
	v.layout.PrintStatus(m)
}

func (v *View) clearStatus(d time.Duration) {
	v.layout.ClearStatus(d)
}

func (v *View) drawScreen(targets []Match) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	if targets == nil {
		if current := v.current; current != nil {
			targets = v.current
		} else {
			targets = v.lines
		}
	}

	v.layout.DrawScreen(targets)
	// FIXME
	v.current = targets
}

func (v *View) movePage(p PagingRequest) {
	_, height := termbox.Size()
	perPage := height - 4

	switch p {
	case ToPrevLine:
		v.currentLine--
	case ToNextLine:
		v.currentLine++
	case ToPrevPage, ToNextPage:
		if p == ToPrevPage {
			v.currentLine -= perPage
		} else {
			v.currentLine += perPage
		}
	}

	if v.currentLine < 1 {
		if v.current != nil {
			// Go to last page, if possible
			v.currentLine = len(v.current)
		} else {
			v.currentLine = 1
		}
	} else if v.current != nil && v.currentLine > len(v.current) {
		v.currentLine = 1
	}
	v.drawScreen(nil)
}
