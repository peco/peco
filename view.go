package peco

import (
	"time"

	"context"
	"github.com/peco/peco/hub"
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
	layout := NewLayout(LayoutType(state.LayoutType()), state)
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
		case r := <-h.StatusMsgCh():
			v.printStatus(r, r.Data().(statusMsgReq))
		case r := <-h.PagingCh():
			v.movePage(r, r.Data().(PagingRequest))
		case r := <-h.DrawCh():
			tmp := r.Data()
			switch tmp := tmp.(type) {
			case string:
				switch tmp {
				case "prompt":
					v.drawPrompt(r)
				case "purgeCache":
					v.purgeDisplayCache(r)
				}
			case *DrawOptions:
				v.drawScreen(r, tmp)
			default:
				v.drawScreen(r, nil)
			}
		}
	}
}

func (v *View) printStatus(p hub.Payload, r statusMsgReq) {
	defer p.Done()
	v.layout.PrintStatus(r.Message(), r.Delay())
}

func (v *View) purgeDisplayCache(p hub.Payload) {
	defer p.Done()

	v.layout.PurgeDisplayCache()
}

func (v *View) drawScreen(p hub.Payload, options *DrawOptions) {
	defer p.Done()

	v.layout.DrawScreen(v.state, options)
}

func (v *View) drawPrompt(p hub.Payload) {
	defer p.Done()

	v.layout.DrawPrompt(v.state)
}

func (v *View) movePage(p hub.Payload, r PagingRequest) {
	defer p.Done()

	if v.layout.MovePage(v.state, r) {
		v.layout.DrawScreen(v.state, nil)
	}
}
