package peco

import (
	"context"
	"fmt"

	"github.com/peco/peco/hub"
)

// View handles the drawing/updating the screen
type View struct {
	layout Layout
	state  *Peco
}

// NewView creates a new View with the given state and its configured layout.
func NewView(state *Peco) (*View, error) {
	layout, err := NewLayout(state.LayoutType(), state)
	if err != nil {
		return nil, fmt.Errorf("failed to create layout: %w", err)
	}
	return &View{
		state:  state,
		layout: layout,
	}, nil
}

// Loop runs the main view loop, listening for draw, paging, and status message
// events from the hub and dispatching them to the appropriate handlers.
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

// printStatus renders a status message on the screen's status bar.
func (v *View) printStatus(p *hub.Payload[hub.StatusMsg]) {
	defer p.Done()
	r := p.Data()
	v.layout.PrintStatus(r.Message(), r.Delay())
}

// purgeDisplayCache clears the cached display state, forcing a full redraw on
// the next draw cycle.
func (v *View) purgeDisplayCache(p *hub.Payload[*hub.DrawOptions]) {
	defer p.Done()

	v.layout.PurgeDisplayCache()
}

// drawScreen renders the current state (prompt, list, status) to the terminal screen.
func (v *View) drawScreen(p *hub.Payload[*hub.DrawOptions], options *hub.DrawOptions) {
	defer p.Done()

	v.layout.DrawScreen(v.state, options)
}

// drawPrompt renders the query prompt line with cursor position.
func (v *View) drawPrompt(p *hub.Payload[*hub.DrawOptions]) {
	defer p.Done()

	v.layout.DrawPrompt(v.state)
}

// movePage handles paging events such as scroll up/down and jump to top/bottom.
func (v *View) movePage(p *hub.Payload[hub.PagingRequest]) {
	defer p.Done()

	r := p.Data()
	if v.layout.MovePage(v.state, r) {
		v.layout.DrawScreen(v.state, nil)
	}
}
