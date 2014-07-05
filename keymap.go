package peco

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/keyseq"
)

// Possible key modifiers
const (
	ModNone = iota
	ModAlt
	ModMax
)

// Keyseq does successive matches against key events.
var Keyseq = keyseq.New()

type Keymap [ModMax]RawKeymap

// RawKeymap contains the actual mapping from termbox.Key to Action
type RawKeymap map[termbox.Key]Action

type KeymapStringKey string

// This map is populated using some magic numbers, which must match
// the values defined in termbox-go. Verification against the actual
// termbox constants are done in the test
var stringToKey = map[string]termbox.Key{}

func init() {
	fidx := 12
	for k := termbox.KeyF1; k > termbox.KeyF12; k-- {
		sk := fmt.Sprintf("F%d", fidx)
		stringToKey[sk] = k
		fidx--
	}

	names := []string{
		"Insert",
		"Delete",
		"Home",
		"End",
		"Pgup",
		"Pgdn",
		"ArrowUp",
		"ArrowDown",
		"ArrowLeft",
		"ArrowRight",
	}
	for i, n := range names {
		stringToKey[n] = termbox.Key(int(termbox.KeyF12) - (i + 1))
	}

	names = []string{
		"Left",
		"Middle",
		"Right",
	}
	for i, n := range names {
		sk := fmt.Sprintf("Mouse%s", n)
		stringToKey[sk] = termbox.Key(int(termbox.KeyArrowRight) - (i + 2))
	}

	whacky := [][]string{
		{"~", "2", "Space"},
		{"a"},
		{"b"},
		{"c"},
		{"d"},
		{"e"},
		{"f"},
		{"g"},
		{"h"},
		{"i"},
		{"j"},
		{"k"},
		{"l"},
		{"m"},
		{"n"},
		{"o"},
		{"p"},
		{"q"},
		{"r"},
		{"s"},
		{"t"},
		{"u"},
		{"v"},
		{"w"},
		{"x"},
		{"y"},
		{"z"},
		{"[", "3"},
		{"4", "\\"},
		{"5", "]"},
		{"6"},
		{"7", "/", "_"},
	}
	for i, list := range whacky {
		for _, n := range list {
			sk := fmt.Sprintf("C-%s", n)
			stringToKey[sk] = termbox.Key(int(termbox.KeyCtrlTilde) + i)
		}
	}

	stringToKey["BS"] = termbox.KeyBackspace
	stringToKey["Tab"] = termbox.KeyTab
	stringToKey["Enter"] = termbox.KeyEnter
	stringToKey["Esc"] = termbox.KeyEsc
	stringToKey["Space"] = termbox.KeySpace
	stringToKey["BS2"] = termbox.KeyBackspace2
	stringToKey["C-8"] = termbox.KeyCtrl8

	//	panic(fmt.Sprintf("%#q", stringToKey))
}

func (ksk KeymapStringKey) ToKeyList() (keyseq.KeyList, error) {
	list := keyseq.KeyList{}
	for _, term := range strings.Split(string(ksk), ",") {
		term = strings.TrimSpace(term)

		k, m, ch, err := KeymapStringKey(term).ToKey()
		if err != nil {
			return list, err
		}

		list = append(list, keyseq.Key{m,k,ch})
	}
	return list, nil
}

func (ksk KeymapStringKey) ToKey() (k termbox.Key, modifier int, ch rune, err error) {
	modifier = ModNone
	key := string(ksk)
	if strings.HasPrefix(key, "M-") {
		modifier = ModAlt
		key = key[2:]
		if len(key) == 1 {
			ch = rune(key[0])
			return
		}
	}

	var ok bool
	k, ok = stringToKey[key]
	if !ok {
		err = fmt.Errorf("No such key %s", ksk)
	}
	return
}

func NewKeymap() Keymap {
	def := RawKeymap{}
	for k, v := range defaultKeyBinding {
		def[k] = v
	}
	return Keymap{
		def,
		{},
	}
}

func (km Keymap) Handler(ev termbox.Event, chained bool) Action {
	modifier := ModNone
	if (ev.Mod & termbox.ModAlt) != 0 {
		modifier = ModAlt
	}

	key := keyseq.Key{modifier,ev.Key,ev.Ch}
	action, err := Keyseq.AcceptKey(key)

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

func (km Keymap) UnmarshalJSON(buf []byte) error {
	raw := map[string]string{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	for ks, vs := range raw {
		list, err := KeymapStringKey(ks).ToKeyList()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown key %s", ks)
			continue
		}

		if vs == "-" {
			// XXX TODO: how do we delete from a trie?
			continue
		}

		v, ok := nameToActions[vs]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown handler '%s'\n", vs)
			continue
		}

		Keyseq.Add(list, v)
	}

	return nil
}

// TODO: this needs to be fixed.
func (km Keymap) hasModifierMaps() bool {
	return len(km[ModAlt]) > 0
}
