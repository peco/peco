package peco

import (
	"time"

	"context"

	"github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/internal/keyseq"
)

func NewInput(state *Peco, am ActionMap, src chan Event) *Input {
	return &Input{
		actions:    am,
		evsrc:      src,
		pendingEsc: make(chan Event, 1),
		state:      state,
	}
}

func (i *Input) Loop(ctx context.Context, cancel func()) error {
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev := <-i.pendingEsc:
			// Timer fired and determined this was a standalone Esc press.
			// Execute the action here on the input loop goroutine, not
			// on the timer goroutine, to avoid concurrent ExecuteAction calls.
			i.state.Keymap().ExecuteAction(ctx, i.state, ev)
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
		// The terminal library doesn't differentiate this for us, so
		// we workaround it by waiting (juuuust a few milliseconds) for
		// extra key events. If no extra events arrive, it should be Esc

		m := &i.mutex

		// Smells like Esc or Alt. mod == nil checks for the presence
		// of a previous timer
		m.Lock()
		if ev.Ch == 0 && ev.Key == 27 && i.mod == nil {
			tmp := ev
			i.modGen++
			gen := i.modGen
			i.mod = time.AfterFunc(50*time.Millisecond, func() {
				m.Lock()
				if i.modGen != gen {
					// A subsequent key event already cancelled this timer.
					// Stop() may have returned false (timer already fired),
					// but the generation was bumped, so we must not execute.
					m.Unlock()
					return
				}
				i.mod = nil
				m.Unlock()
				// Send to the input loop instead of calling ExecuteAction
				// directly, so all action execution is serialized on the
				// input loop goroutine.
				select {
				case i.pendingEsc <- tmp:
				case <-ctx.Done():
				}
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
			i.modGen++ // invalidate any pending timer callback
			i.mod = nil
			ev.Mod = keyseq.ModAlt
		}
		m.Unlock()

		i.state.Keymap().ExecuteAction(ctx, i.state, ev)

		return nil
	}

	return nil
}
