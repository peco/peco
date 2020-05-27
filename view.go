package peco

import (
	"time"

	"context"

	"github.com/lestrrat-go/pdebug/v2"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/ui"
)

type statusMsgReq interface {
	Message() string
	Delay() time.Duration
}

func NewView(state *Peco) *View {
	var layout ui.Layout
	switch state.LayoutType() {
	case ui.LayoutTypeBottomUp:
		layout = ui.NewBottomUpLayout(state)
	default:
		layout = ui.NewDefaultLayout(state.Screen(), state.Styles(), state.Prompt())
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
			v.movePage(r, r.Data().(ui.PagingRequest))
		case r := <-h.DrawCh():
			switch tmp := r.Data().(type) {
			case string:
				if tmp == "prompt" {
					v.drawPrompt(ctx, r)
				} else if tmp == "purgeCache" {
					v.purgeDisplayCache(r)
				}
// TODO
//			case *DrawOptions:
//				v.drawScreen(r, tmp)
			default:
				v.drawScreen(r)
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

func (v *View) drawScreen(p hub.Payload, options ...ui.Option) {
	defer p.Done()

	ctx := context.TODO()
	if pdebug.Enabled {
		ctx = pdebug.Context(ctx)
	}
	v.layout.DrawScreen(ctx, v.state, options...)
}

func (v *View) drawPrompt(ctx context.Context, p hub.Payload) {
	defer p.Done()

	v.layout.DrawPrompt(ctx, v.state)
}

func (v *View) movePage(p hub.Payload, r ui.PagingRequest) {
	defer p.Done()

	ctx := context.TODO()
	if pdebug.Enabled {
		ctx = pdebug.Context(ctx)
	}
	if v.layout.MovePage(v.state, r) {
		v.layout.DrawScreen(ctx, v.state)
	}
}
