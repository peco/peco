package peco

import (
	"sync"
	"time"

	"golang.org/x/net/context"
)

const (
	// ToLineAbove moves the selection to the line above
	ToLineAbove PagingRequestType = iota	// ToScrollPageDown moves the selection to the next page
	ToScrollPageDown
	// ToLineBelow moves the selection to the line below
	ToLineBelow
	// ToScrollPageUp moves the selection to the previous page
	ToScrollPageUp
	// ToScrollLeft scrolls screen to the left
	ToScrollLeft
	// ToScrollRight scrolls screen to the right
	ToScrollRight
	// ToLineInPage jumps to a particular line on the page
	ToLineInPage
)

type View struct {
	mutex  sync.Mutex
	layout Layout
	state *Peco
}

type statusMsgReq interface {
	Message() string
	Delay() time.Duration
}

func (prt PagingRequestType) Type() PagingRequestType {
	return prt
}

func (jlr JumpToLineRequest) Type() PagingRequestType {
	return ToLineInPage
}

func (jlr JumpToLineRequest) Line() int {
	return int(jlr)
}

/*
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
				switch tmp.(string) {
				case "prompt":
					v.drawPrompt()
				case "purgeCache":
					v.purgeDisplayCache()
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

func (v *View) purgeDisplayCache() {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.layout.PurgeDisplayCache()
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
*/

func NewView(state *Peco) *View {
	return &View{
		state: state,
	}
}

func (v *View) Loop(ctx context.Context, cancel func()) error {
	defer cancel()

	h := v.state.Hub()
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-h.StatusMsgCh():
			v.printStatus(m.Data().(statusMsgReq))
			m.Done()
		case r := <-h.PagingCh():
			v.movePage(r.Data().(PagingRequest))
			r.Done()
		case lines := <-h.DrawCh():
			tmp := lines.Data()
			switch tmp.(type) {
			case string:
				switch tmp.(string) {
				case "prompt":
					v.drawPrompt()
				case "purgeCache":
					v.purgeDisplayCache()
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

func (v *View) printStatus(r statusMsgReq) {
	v.layout.PrintStatus(r.Message(), r.Delay())
}

func (v *View) purgeDisplayCache() {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.layout.PurgeDisplayCache()
}

func (v *View) drawScreen(runningQuery bool) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.layout.DrawScreen(v.state, runningQuery)
}

func (v *View) drawPrompt() {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.layout.DrawPrompt(v.state)
}

func (v *View) movePage(p PagingRequest) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	if v.layout.MovePage(v.state, p) {
		v.layout.DrawScreen(v.state, false)
	}
}
