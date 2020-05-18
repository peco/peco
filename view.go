package peco

import (
	"time"

	"context"

	"github.com/lestrrat-go/pdebug/v2"
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
		case r := <-h.StatusMsgCh():
			v.printStatus(r, r.Data().(statusMsgReq))
		case r := <-h.PagingCh():
			v.movePage(r, r.Data().(PagingRequest))
		case r := <-h.DrawCh():
			switch tmp := r.Data().(type) {
			case string:
				if tmp == "prompt" {
					v.drawPrompt(ctx, r)
				} else if tmp == "purgeCache" {
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

	ctx := context.TODO()
	if pdebug.Enabled {
		ctx = pdebug.Context(ctx)
	}
	v.layout.DrawScreen(ctx, v.state, options)
}

func (v *View) drawPrompt(ctx context.Context, p hub.Payload) {
	defer p.Done()

	v.layout.DrawPrompt(ctx, v.state)
}

func (v *View) movePage(p hub.Payload, r PagingRequest) {
	defer p.Done()

	ctx := context.TODO()
	if pdebug.Enabled {
		ctx = pdebug.Context(ctx)
	}
	if v.layout.MovePage(v.state, r) {
		v.layout.DrawScreen(ctx, v.state, nil)
	}
}
