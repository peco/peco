package peco


/*

import (
	"time"

	"github.com/nsf/termbox-go"
)

// Loop watches for incoming events from termbox, and pass them
// to the appropriate handler when something arrives.
func (i *Input) Loop() {
	trace("Input.Loop: START")
	defer trace("Input.Loop: END")
	defer i.ReleaseWaitGroup()

	evCh := screen.PollEvent()

	for {
		select {
		case <-i.LoopCh(): // can only fall here if we closed c.loopCh
			return
		case ev := <-evCh:
			i.handleInputEvent(ev)
		}
	}
}

func (i *Input) handleInputEvent(ev termbox.Event) {
	switch ev.Type {
	case termbox.EventError:
		//update = false
	case termbox.EventResize:
		i.SendDraw(false)
	case termbox.EventKey:
		// ModAlt is a sequence of letters with a leading \x1b (=Esc).
		// It would be nice if termbox differentiated this for us, but
		// we workaround it by waiting (juuuuse a few milliseconds) for
		// extra key events. If no extra events arrive, it should be Esc

		// Smells like Esc or Alt. mod == nil checks for the presense
		// of a previous timer
		if ev.Ch == 0 && ev.Key == 27 && i.mod == nil {
			tmp := ev
			i.mutex.Lock()
			i.mod = time.AfterFunc(50*time.Millisecond, func() {
				i.mutex.Lock()
				i.mod = nil
				i.mutex.Unlock()
				trace("Input.handleInputEvent: Firing delayed input event")
				i.handleKeyEvent(tmp)
			})
			i.mutex.Unlock()
		} else {
			// it doesn't look like this is Esc or Alt. If we have a previous
			// timer, stop it because this is probably Alt+ this new key
			i.mutex.Lock()
			if i.mod != nil {
				i.mod.Stop()
				i.mod = nil
				ev.Mod |= termbox.ModAlt
			}
			i.mutex.Unlock()
			trace("Input.handleInputEvent: Firing event")
			i.handleKeyEvent(ev)
		}
	}
}

func (i *Input) handleKeyEvent(ev termbox.Event) {
	trace("Input.handleKeyEvent: START")
	defer trace("Input.handleKeyEvent: END")
	if a := i.keymap.LookupAction(ev); a != nil {
		trace("Input.handleKeyEvent: Event %#v maps to %s, firing action", ev, a)
		a.Execute(i, ev)
		return
	}
}
*/

import (
	"sync"
	"time"

	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type Redrawer interface {
	Redraw(bool)
}

type ActionMap interface {
	ExecuteAction(context.Context, termbox.Event) error
}

type Input struct {
	actions ActionMap
	evsrc   chan termbox.Event
	mod     *time.Timer
	mutex   sync.Mutex
	state   *Peco
}

func NewInput(state *Peco, am ActionMap, src chan termbox.Event) *Input {
	return &Input{
		actions: am,
		evsrc:   src,
		mutex:   sync.Mutex{},
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
	return nil
}

func (i *Input) handleInputEvent(ctx context.Context, ev termbox.Event) error {
	switch ev.Type {
	case termbox.EventError:
		return nil
	case termbox.EventResize:
		return errors.New("redraw unimplemented")
		//		p.Redraw(false)
		//		return nil
	case termbox.EventKey:
		// ModAlt is a sequence of letters with a leading \x1b (=Esc).
		// It would be nice if termbox differentiated this for us, but
		// we workaround it by waiting (juuuuse a few milliseconds) for
		// extra key events. If no extra events arrive, it should be Esc

		m := i.mutex

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

		i.actions.ExecuteAction(ctx, ev)

		return nil
	}

	return nil
}

