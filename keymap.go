package peco

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/internal/keyseq"
)

// NewKeymap creates a new Keymap struct
func NewKeymap(state *Peco, config map[string]string, actions map[string][]string) Keymap {
	return Keymap{
		Config: config,
		Action: actions,
		seq:    keyseq.New(),
		state:  state,
	}
}

func (km Keymap) Sequence() Keyseq {
	return km.seq
}

func (km Keymap) ExecuteAction(ctx context.Context, ev termbox.Event) error {
	a := km.LookupAction(ev)
	if a == nil {
		return errors.New("action not found")
	}

	a.(Action).Execute(ctx, km.state, ev)
	return nil
}

// LookupAction returns the appropriate action for the given termbox event
func (km Keymap) LookupAction(ev termbox.Event) Action {
	modifier := keyseq.ModNone
	if (ev.Mod & termbox.ModAlt) != 0 {
		modifier = keyseq.ModAlt
	}

	key := keyseq.Key{modifier, ev.Key, ev.Ch}
	action, err := km.seq.AcceptKey(key)

	switch err {
	case nil:
		// Found an action!
		trace("Keymap.Handler: Fetched action")
		return wrapClearSequence(action.(Action))
	case keyseq.ErrInSequence:
		trace("Keymap.Handler: Waiting for more commands...")
		return wrapRememberSequence(ActionFunc(doNothing))
	default:
		trace("Keymap.Handler: Defaulting to doAcceptChar")
		return wrapClearSequence(ActionFunc(doAcceptChar))
	}
}

func wrapRememberSequence(a Action) Action {
	return ActionFunc(func(ctx context.Context, state *Peco, ev termbox.Event) {
		if s, err := keyseq.EventToString(ev); err == nil {
			seq := state.Inputseq()
			seq.Add(s)
			state.Hub().SendStatusMsg(strings.Join(seq.KeyNames(), " "))
		}
		a.Execute(ctx, state, ev)
	})
}

func wrapClearSequence(a Action) Action {
	return ActionFunc(func(ctx context.Context, state *Peco, ev termbox.Event) {
		seq := state.Inputseq()
		if s, err := keyseq.EventToString(ev); err == nil {
			seq.Add(s)
		}

		if seq.Len() > 0 {
			msg := strings.Join(seq.KeyNames(), " ")
			state.Hub().SendStatusMsgAndClear(msg, 500*time.Millisecond)
			seq.Reset()
		}

		a.Execute(ctx, state, ev)
	})
}

const maxResolveActionDepth = 100

func (km Keymap) resolveActionName(name string, depth int) (Action, error) {
	if depth >= maxResolveActionDepth {
		return nil, fmt.Errorf("error: Could not resolve %s: deep recursion", name)
	}

	// Can it be resolved via regular nameToActions ?
	v, ok := nameToActions[name]
	if ok {
		return v, nil
	}

	// Can it be resolved via combined actions?
	l, ok := km.Action[name]
	if ok {
		actions := []Action{}
		for _, actionName := range l {
			child, err := km.resolveActionName(actionName, depth+1)
			if err != nil {
				return nil, err
			}
			actions = append(actions, child)
		}
		v = makeCombinedAction(actions...)
		nameToActions[name] = v
		return v, nil
	}

	return nil, fmt.Errorf("error: Could not resolve %s: no such action", name)
}

// ApplyKeybinding applies all of the custom key bindings on top of
// the default key bindings
func (km Keymap) ApplyKeybinding() {
	k := km.seq
	k.Clear()

	// Copy the map
	kb := map[string]Action{}
	for s, a := range defaultKeyBinding {
		kb[s] = a
	}

	// munge the map using config
	for s, as := range km.Config {
		if as == "-" {
			delete(kb, s)
			continue
		}

		v, err := km.resolveActionName(as, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		kb[s] = v
	}

	// now compile using kb
	for s, a := range kb {
		list, err := keyseq.ToKeyList(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown key %s: %s", s, err)
			continue
		}

		k.Add(list, a)
	}

	k.Compile()
}

// TODO: this needs to be fixed.
func (km Keymap) hasModifierMaps() bool {
	return false
}
