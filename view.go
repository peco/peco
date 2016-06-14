package peco

import (
	"time"

	"golang.org/x/net/context"
)

const (
	// ToLineAbove moves the selection to the line above
	ToLineAbove PagingRequestType = iota // ToScrollPageDown moves the selection to the next page
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

func NewView(state *Peco) *View {
	var layout Layout
	switch state.LayoutType() {
	case LayoutTypeBottomUp:
		layout = NewBottomUpLayout(state)
	default:
		layout = NewDefaultLayout(state)
	}
	return &View{
		state:  state,
		layout: layout,
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

	trace("Pageing request = %s", p)
	if v.layout.MovePage(v.state, p) {
		v.layout.DrawScreen(v.state, false)
	}
}
