package peco

import (
	"time"

	"context"

	"github.com/lestrrat-go/pdebug/v2"
	"github.com/nsf/termbox-go"
)

func NewInput(state *Peco, am ActionMap, src chan termbox.Event) *Input {
	return &Input{
		actions: am,
		evsrc:   src,
		state:   state,
	}
}

func (i *Input) Loop(ctx context.Context, cancel func()) error {
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-i.evsrc:
			if err := i.handleInputEvent(ctx, ev); err != nil {
				return nil
			}
		}
	}
}

func (i *Input) handleInputEvent(ctx context.Context, ev termbox.Event) error {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "event received from user: %#v", ev)
		defer g.End()
	}

	switch ev.Type {
	case termbox.EventError:
		return nil
	case termbox.EventResize:
		i.state.Hub().SendDraw(ctx, nil)
		return nil
	case termbox.EventKey:
		// ModAlt is a sequence of letters with a leading \x1b (=Esc).
		// It would be nice if termbox differentiated this for us, but
		// we workaround it by waiting (juuuust a few milliseconds) for
		// extra key events. If no extra events arrive, it should be Esc

		m := &i.mutex

		// Smells like Esc or Alt. mod == nil checks for the presence
		// of a previous timer
		m.Lock()
		if ev.Ch == 0 && ev.Key == 27 && i.mod == nil {
			tmp := ev
			i.mod = time.AfterFunc(50*time.Millisecond, func() {
				m.Lock()
				i.mod = nil
				m.Unlock()
				_ = i.state.Keymap().ExecuteAction(ctx, i.state, tmp)
			})
			m.Unlock()
			return nil
		}
		m.Unlock()

		// it doesn't look like this is Esc or Alt. If we have a previous
		// timer, stop it because this is probably Alt+ this new key
		m.Lock()
		if i.mod != nil {
			i.mod.Stop()
			i.mod = nil
			ev.Mod |= termbox.ModAlt
		}
		m.Unlock()

		return i.state.Keymap().ExecuteAction(ctx, i.state, ev)
	}

	return nil
}
