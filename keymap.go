package peco

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/keyseq"
)

type Keymap struct {
	Config map[string]string
	Keyseq *keyseq.Keyseq
}

func NewKeymap(config map[string]string) Keymap {
	return Keymap{config, keyseq.New()}

}

func (km Keymap) Handler(ev termbox.Event) Action {
	modifier := keyseq.ModNone
	if (ev.Mod & termbox.ModAlt) != 0 {
		modifier = keyseq.ModAlt
	}

	key := keyseq.Key{modifier, ev.Key, ev.Ch}
	action, err := km.Keyseq.AcceptKey(key)

	switch err {
	case nil:
		// Found an action!
		return action.(Action)
	case keyseq.ErrInSequence:
		// TODO We're in some sort of key sequence. Remember what we have
		// received so far
		return ActionFunc(doNothing)
	default:
		return ActionFunc(doAcceptChar)
	}
}

func (km Keymap) resolveActionName(name string) (Action, error) {
	// try direct lookup
	action, ok := nameToActions[name]
	if ok && action != nil {
		return action, nil
	}

	// If all else fails...
	// Finish is a special case. We can dynamically create  a finish
	// function that exits with an arbitrary exit status
	if strings.HasPrefix(name, "peco.Finish") {
		v, err := strconv.ParseInt(name[11:], 10, 64)
		if err == nil {
			action = makeFinishAction(int(v))
			nameToActions[name] = action
			return action, nil
		}
	}

	// Nothing found, exit
	return nil, fmt.Errorf("No action found for %s", name)
}

func (km Keymap) ApplyKeybinding() {
	k := km.Keyseq
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

		v, err := km.resolveActionName(as)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve action '%s': %s", as, err)
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
