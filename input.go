package peco

import (
	"context"

	"github.com/lestrrat-go/pdebug"
)

// Input handles dispatching key events to actions.
type Input struct {
	actions ActionMap
	evsrc   chan Event
	state   *Peco
}

// NewInput creates and returns a new Input instance for handling keyboard events.
func NewInput(state *Peco, am ActionMap, src chan Event) *Input {
	return &Input{
		actions: am,
		evsrc:   src,
		state:   state,
	}
}

// Loop runs the main input event loop, reading terminal events and dispatching them.
func (i *Input) Loop(ctx context.Context, cancel func()) error {
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-i.evsrc:
			if !ok {
				return nil
			}
			if err := i.handleInputEvent(ctx, ev); err != nil {
				return nil
			}
		}
	}
}

// handleInputEvent processes a single terminal input event, mapping it to an action.
func (i *Input) handleInputEvent(ctx context.Context, ev Event) error {
	if pdebug.Enabled {
		g := pdebug.Marker("event received from user: %#v", ev)
		defer g.End()
	}

	switch ev.Type {
	case EventError:
		return nil
	case EventResize:
		i.state.Hub().SendDraw(ctx, nil)
		return nil
	case EventKey:
		// tcell already disambiguates standalone Escape from Alt+key
		// sequences using its own internal escape timer. By the time
		// we receive a KeyEsc event here, tcell has already determined
		// it is a standalone Escape. Alt+key events arrive with ModAlt
		// already set. No application-layer timer is needed.
		_ = i.state.Keymap().ExecuteAction(ctx, i.state, ev)
		return nil
	}

	return nil
}
