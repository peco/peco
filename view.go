package peco

import (
	"context"
	"fmt"

	"github.com/peco/peco/hub"
)

func NewView(state *Peco) (*View, error) {
	layout, err := NewLayout(LayoutType(state.LayoutType()), state)
	if err != nil {
		return nil, fmt.Errorf("failed to create layout: %w", err)
	}
	return &View{
		state:  state,
		layout: layout,
	}, nil
}

func (v *View) Loop(ctx context.Context, cancel func()) error {
	defer cancel()

	h := v.state.Hub()
	for {
		select {
		case <-ctx.Done():
			return nil
		case r := <-h.StatusMsgCh():
			v.printStatus(r)
		case r := <-h.PagingCh():
			v.movePage(r)
		case r := <-h.DrawCh():
			opts := r.Data()
			switch {
			case opts != nil && opts.Prompt:
				v.drawPrompt(r)
			case opts != nil && opts.PurgeCache:
				v.purgeDisplayCache(r)
			default:
				v.drawScreen(r, opts)
			}
		}
	}
}

func (v *View) printStatus(p *hub.Payload[hub.StatusMsg]) {
	defer p.Done()
	r := p.Data()
	v.layout.PrintStatus(r.Message(), r.Delay())
}

func (v *View) purgeDisplayCache(p *hub.Payload[*hub.DrawOptions]) {
	defer p.Done()

	v.layout.PurgeDisplayCache()
}

func (v *View) drawScreen(p *hub.Payload[*hub.DrawOptions], options *hub.DrawOptions) {
	defer p.Done()

	v.layout.DrawScreen(v.state, options)
}

func (v *View) drawPrompt(p *hub.Payload[*hub.DrawOptions]) {
	defer p.Done()

	v.layout.DrawPrompt(v.state)
}

func (v *View) movePage(p *hub.Payload[hub.PagingRequest]) {
	defer p.Done()

	r := p.Data()
	if v.layout.MovePage(v.state, r) {
		v.layout.DrawScreen(v.state, nil)
	}
}
