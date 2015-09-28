package peco

const (
	// ToLineAbove moves the selection to the line above
	ToLineAbove PagingRequest = iota
	// ToScrollPageDown moves the selection to the next page
	ToScrollPageDown
	// ToLineBelow moves the selection to the line below
	ToLineBelow
	// ToScrollPageUp moves the selection to the previous page
	ToScrollPageUp
	// ToScrollLeft scrolls screen to the left
	ToScrollLeft
	// ToScrollRight scrolls screen to the right
	ToScrollRight
)

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
			switch tmp.(type) {
			case string:
				if tmp.(string) == "prompt" {
					v.drawPrompt()
				}
			case bool:
				v.drawScreen(tmp.(bool))
			default:
				v.drawScreen(false)
			}
			lines.Done()
		}
	}
}

func (v *View) printStatus(r StatusMsgRequest) {
	v.layout.PrintStatus(r.message, r.clearDelay)
}

func (v *View) drawScreen(runningQuery bool) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.layout.DrawScreen(runningQuery)
}

func (v *View) drawPrompt() {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.layout.DrawPrompt()
}

func (v *View) movePage(p PagingRequest) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	if v.layout.MovePage(p) {
		v.layout.DrawScreen(false)
	}
}
