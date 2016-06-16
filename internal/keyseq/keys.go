package keyseq

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
)

// This map is populated using some magic numbers, which must match
// the values defined in termbox-go. Verification against the actual
// termbox constants are done in the test
var stringToKey = map[string]termbox.Key{}
var keyToString = map[termbox.Key]string{}

func mapkey(n string, k termbox.Key) {
	stringToKey[n] = k
	keyToString[k] = n
}

func init() {
	fidx := 1
	for k := termbox.KeyF1; k >= termbox.KeyF12; k-- {
		sk := fmt.Sprintf("F%d", fidx)
		mapkey(sk, k)
		fidx++
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
		mapkey(n, termbox.Key(int(termbox.KeyF12)-(i+1)))
	}

	names = []string{
		"Left",
		"Middle",
		"Right",
	}
	for i, n := range names {
		sk := fmt.Sprintf("Mouse%s", n)
		mapkey(sk, termbox.Key(int(termbox.KeyArrowRight)-(i+2)))
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
			mapkey(sk, termbox.Key(int(termbox.KeyCtrlTilde)+i))
		}
	}

	mapkey("BS", termbox.KeyBackspace)
	mapkey("Tab", termbox.KeyTab)
	mapkey("Enter", termbox.KeyEnter)
	mapkey("Esc", termbox.KeyEsc)
	mapkey("Space", termbox.KeySpace)
	mapkey("BS2", termbox.KeyBackspace2)
	mapkey("C-8", termbox.KeyCtrl8)

	//	panic(fmt.Sprintf("%#q", stringToKey))
}

func ToKeyList(ksk string) (KeyList, error) {
	list := KeyList{}
	for _, term := range strings.Split(ksk, ",") {
		term = strings.TrimSpace(term)

		k, m, ch, err := ToKey(term)
		if err != nil {
			return list, errors.Wrapf(err, "failed to convert '%s'", term)
		}

		list = append(list, Key{m, k, ch})
	}
	return list, nil
}

// EventToString returns human readable name for a given termbox.Event
func EventToString(ev termbox.Event) (string, error) {
	s := ""
	if ev.Key == 0 {
		s = string([]rune{ev.Ch})
	} else {
		var ok bool
		s, ok = keyToString[ev.Key]
		if !ok {
			return "", errors.Errorf("no such key %#v", ev)
		}

		// Special case for ArrowUp/Down/Left/Right
		switch s {
		case "ArrowUp":
			s = "^"
		case "ArrowDown":
			s = "v"
		case "ArrowLeft":
			s = "<"
		case "ArrowRight":
			s = ">"
		}
	}

	if ev.Mod&termbox.ModAlt == 1 {
		return "M-" + s, nil
	}

	return s, nil
}

func ToKey(key string) (k termbox.Key, modifier ModifierKey, ch rune, err error) {
	modifier = ModNone
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
		// If this is a single rune, just allow it
		ch, _ = utf8.DecodeRuneInString(key)
		if ch != utf8.RuneError {
			return
		}

		err = errors.Errorf("no such key %s", key)
	}
	return
}
