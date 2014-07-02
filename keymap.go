package peco

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/nsf/termbox-go"
)

// Possible key modifiers
const (
	ModNone = iota
	ModAlt
	ModMax
)

// Keymap contains keys which are modifiers (like Alt+X), and points to
// RawKeymap
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

func handleAcceptChar(i *Input, ev termbox.Event) {
	if ev.Key == termbox.KeySpace {
		ev.Ch = ' '
	}

	if ev.Ch > 0 {
		if len(i.query) == i.caretPos {
			i.query = append(i.query, ev.Ch)
		} else {
			buf := make([]rune, len(i.query)+1)
			copy(buf, i.query[:i.caretPos])
			buf[i.caretPos] = ev.Ch
			copy(buf[i.caretPos+1:], i.query[i.caretPos:])
			i.query = buf
		}
		i.caretPos++
		i.ExecQuery()
	}
}

func (ksk KeymapStringKey) ToKey() (k termbox.Key, modifier int, err error) {
	modifier = ModNone
	key := string(ksk)
	if strings.HasPrefix(key, "M-") {
		modifier = ModAlt
		key = key[2:]
		if len(key) == 1 {
			k = termbox.Key(key[0])
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

func (km Keymap) Handler(ev termbox.Event) Action {
	modifier := ModNone
	if (ev.Mod & termbox.ModAlt) != 0 {
		modifier = ModAlt
	}

	// RawKeymap that we will be using
	rkm := km[modifier]

	switch modifier {
	case ModAlt:
		var key termbox.Key
		if ev.Ch == 0 {
			key = ev.Key
		} else {
			key = termbox.Key(ev.Ch)
		}

		if h, ok := rkm[key]; ok {
			return h
		}
	case ModNone:
		if ev.Ch == 0 {
			if h, ok := rkm[ev.Key]; ok {
				return h
			}
		}
	default:
		panic("Can't get here")
	}

	return ActionFunc(handleAcceptChar)
}

func (km Keymap) UnmarshalJSON(buf []byte) error {
	raw := map[string]string{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}

	for ks, vs := range raw {
		k, modifier, err := KeymapStringKey(ks).ToKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unknown key %s", ks)
			continue
		}

		keymap := km[modifier]
		if vs == "-" {
			delete(keymap, k)
			continue
		}

		v, ok := handlers[vs]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown handler %s", vs)
			continue
		}
		keymap[k] = v
	}

	return nil
}

func (km Keymap) hasModifierMaps() bool {
	return len(km[ModAlt]) > 0
}
