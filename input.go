package peco

import (
	"sync"
	"time"

	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
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
	switch ev.Type {
	case termbox.EventError:
		return nil
	case termbox.EventResize:
		i.state.Hub().SendDraw(false)
		return nil
	case termbox.EventKey:
		// ModAlt is a sequence of letters with a leading \x1b (=Esc).
		// It would be nice if termbox differentiated this for us, but
		// we workaround it by waiting (juuuust a few milliseconds) for
		// extra key events. If no extra events arrive, it should be Esc

		m := &i.mutex

		// Smells like Esc or Alt. mod == nil checks for the presense
		// of a previous timer
		if ev.Ch == 0 && ev.Key == 27 && i.mod == nil {
			tmp := ev
			m.Lock()
			i.mod = time.AfterFunc(50*time.Millisecond, func() {
				m.Lock()
				i.mod = nil
				m.Unlock()
				//			trace("Input.handleInputEvent: Firing delayed input event")
				i.handleInputEvent(ctx, tmp)
			})
			m.Unlock()
			return nil
		}

		// it doesn't look like this is Esc or Alt. If we have a previous
		// timer, stop it because this is probably Alt+ this new key
		m.Lock()
		if i.mod != nil {
			i.mod.Stop()
			i.mod = nil
			ev.Mod |= termbox.ModAlt
		}
		m.Unlock()

		i.state.Keymap().ExecuteAction(ctx, i.state, ev)

		return nil
	}

	return nil
}

