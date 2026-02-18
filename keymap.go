package peco

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/internal/keyseq"
)

// NewKeymap creates a new Keymap struct
func NewKeymap(config map[string]string, actions map[string][]string) Keymap {
	return Keymap{
		Config: config,
		Action: actions,
		seq:    keyseq.New(),
	}
}

func (km Keymap) Sequence() Keyseq {
	return km.seq
}

func (km Keymap) ExecuteAction(ctx context.Context, state *Peco, ev Event) (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Keymap.ExecuteAction %v", ev).BindError(&err)
		defer g.End()
	}

	a := km.LookupAction(ev)
	if a == nil {
		return errors.New("action not found")
	}

	a.Execute(ctx, state, ev)
	return nil
}

// LookupAction returns the appropriate action for the given event
func (km Keymap) LookupAction(ev Event) Action {
	key := keyseq.Key{
		Modifier: ev.Mod,
		Key:      ev.Key,
		Ch:       ev.Ch,
	}
	action, err := km.seq.AcceptKey(key)

	switch err {
	case nil:
		// Found an action!
		if pdebug.Enabled {
			pdebug.Printf("Keymap.Handler: Fetched action")
		}
		a, ok := action.(Action)
		if !ok {
			return ActionFunc(doNothing)
		}
		return wrapClearSequence(a)
	case keyseq.ErrInSequence:
		if pdebug.Enabled {
			pdebug.Printf("Keymap.Handler: Waiting for more commands...")
		}
		return wrapRememberSequence(ActionFunc(doNothing))
	default:
		if pdebug.Enabled {
			pdebug.Printf("Keymap.Handler: Defaulting to doAcceptChar")
		}
		return wrapClearSequence(ActionFunc(doAcceptChar))
	}
}

func wrapRememberSequence(a Action) Action {
	return ActionFunc(func(ctx context.Context, state *Peco, ev Event) {
		if s, err := keyseq.KeyEventToString(ev.Key, ev.Ch, ev.Mod); err == nil {
			seq := state.Inputseq()
			seq.Add(s)
			state.Hub().SendStatusMsg(ctx, strings.Join(seq.KeyNames(), " "), 0)
		}
		a.Execute(ctx, state, ev)
	})
}

func wrapClearSequence(a Action) Action {
	return ActionFunc(func(ctx context.Context, state *Peco, ev Event) {
		seq := state.Inputseq()
		if s, err := keyseq.KeyEventToString(ev.Key, ev.Ch, ev.Mod); err == nil {
			seq.Add(s)
		}

		if seq.Len() > 0 {
			msg := strings.Join(seq.KeyNames(), " ")
			state.Hub().SendStatusMsg(ctx, msg, 500*time.Millisecond)
			seq.Reset()
		}

		a.Execute(ctx, state, ev)
	})
}

const maxResolveActionDepth = 100

func (km Keymap) resolveActionName(name string, depth int) (Action, error) {
	if depth >= maxResolveActionDepth {
		return nil, fmt.Errorf("could not resolve %s: deep recursion", name)
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
		return v, nil
	}

	return nil, fmt.Errorf("could not resolve %s: no such action", name)
}

// ApplyKeybinding applies all of the custom key bindings on top of
// the default key bindings
func (km *Keymap) ApplyKeybinding() error {
	k := km.seq
	k.Clear()

	// Copy the map
	kb := map[string]Action{}
	maps.Copy(kb, defaultKeyBinding)

	// munge the map using config
	for s, as := range km.Config {
		if as == "-" {
			delete(kb, s)
			continue
		}

		v, err := km.resolveActionName(as, 0)
		if err != nil {
			return fmt.Errorf("failed to resolve action name %s: %w", as, err)
		}
		kb[s] = v
	}

	// now compile using kb
	// there's no need to do this, but we sort keys here just to make
	// debugging easier
	keys := make([]string, 0, len(kb))
	for s := range kb {
		keys = append(keys, s)
	}
	sort.Strings(keys)

	for _, s := range keys {
		a := kb[s]
		list, err := keyseq.ToKeyList(s)
		if err != nil {
			return fmt.Errorf("unknown key %s: %w", s, err)
		}

		k.Add(list, a)
	}

	if err := k.Compile(); err != nil {
		return fmt.Errorf("failed to compile key binding patterns: %w", err)
	}
	return nil
}
