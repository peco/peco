package peco

import (
	"time"

	"context"

	"github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/internal/keyseq"
)

func NewInput(state *Peco, am ActionMap, src chan Event) *Input {
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
				i.state.Keymap().ExecuteAction(ctx, i.state, tmp)
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
			ev.Mod = keyseq.ModAlt
		}
		m.Unlock()

		i.state.Keymap().ExecuteAction(ctx, i.state, ev)

		return nil
	}

	return nil
}
