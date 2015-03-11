package peco

import (
	"sync"
	"time"
)

// View handles the drawing/updating the screen
type View struct {
	*Ctx
	mutex  sync.Locker
	layout Layout
}

// PagingRequest can be sent to move the selection cursor
type PagingRequest int

const (
	// ToLineAbove moves the selection to the line above
	ToLineAbove PagingRequest = iota
	// ToScrollPageDown moves the selection to the next page
	ToScrollPageDown
	// ToLineBelow moves the selection to the line below
	ToLineBelow
	// ToScrollPageUp moves the selection to the previous page
	ToScrollPageUp
)

// StatusMsgRequest specifies the string to be drawn
// on the status message bar and an optional delay that tells
// the view to clear that message
type StatusMsgRequest struct {
	message    string
	clearDelay time.Duration
}

// Loop receives requests to update the screen
func (v *View) Loop() {
	defer v.ReleaseWaitGroup()
	for {
		select {
		case <-v.LoopCh():
			return
		case m := <-v.StatusMsgCh():
			v.printStatus(m.DataInterface().(StatusMsgRequest))
			m.Done()
		case r := <-v.PagingCh():
			v.movePage(r.DataInterface().(PagingRequest))
			r.Done()
		case lines := <-v.DrawCh():
			tmp := lines.DataInterface()
			if tmp == nil {
				v.drawScreen(nil)
			} else if matches, ok := tmp.([]Line); ok {
				v.drawScreen(matches)
			} else if name, ok := tmp.(string); ok {
				if name == "prompt" {
					v.drawPrompt()
				}
			}
			lines.Done()
		}
	}
}

func (v *View) printStatus(r StatusMsgRequest) {
	v.layout.PrintStatus(r.message, r.clearDelay)
}

func (v *View) drawScreenNoLock(targets []Line) {
	if targets == nil {
		if current := v.GetCurrent(); current != nil {
			targets = current
		} else {
			targets = v.GetLines()
		}
	}

	v.layout.DrawScreen()
}

func (v *View) drawScreen(targets []Line) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.drawScreenNoLock(targets)
}

func (v *View) drawPrompt() {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.layout.DrawPrompt()
}

func (v *View) movePage(p PagingRequest) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	v.layout.MovePage(p)
	v.drawScreenNoLock(nil)
}
