package keyseq

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pkg/errors"
)

// KeyType represents a keyboard key. Values are defined to match termbox-go's
// constants so that the adapter layer in screen.go can do simple type casts
// during Phase 1 of the migration. In Phase 2, the adapter will map between
// peco's KeyType and tcell's key constants.
type KeyType uint16

// Function keys
const (
	KeyF1  KeyType = 0xFFFF - iota
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyInsert
	KeyDelete
	KeyHome
	KeyEnd
	KeyPgup
	KeyPgdn
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
)

// Mouse keys.
// In termbox-go there is one internal gap constant between KeyArrowRight
// and MouseLeft, so MouseLeft = KeyArrowRight - 2 (not -1).
const (
	MouseLeft   KeyType = 0xFFFF - iota - 23 // KeyArrowRight - 2
	MouseMiddle                               // KeyArrowRight - 3
	MouseRight                                // KeyArrowRight - 4
)

// Ctrl keys and special keys.
// These correspond to ASCII control codes.
const (
	KeyCtrlTilde      KeyType = 0x00
	KeyCtrl2          KeyType = 0x00
	KeyCtrlSpace      KeyType = 0x00
	KeyCtrlA          KeyType = 0x01
	KeyCtrlB          KeyType = 0x02
	KeyCtrlC          KeyType = 0x03
	KeyCtrlD          KeyType = 0x04
	KeyCtrlE          KeyType = 0x05
	KeyCtrlF          KeyType = 0x06
	KeyCtrlG          KeyType = 0x07
	KeyCtrlH          KeyType = 0x08
	KeyCtrlI          KeyType = 0x09
	KeyCtrlJ          KeyType = 0x0A
	KeyCtrlK          KeyType = 0x0B
	KeyCtrlL          KeyType = 0x0C
	KeyCtrlM          KeyType = 0x0D
	KeyCtrlN          KeyType = 0x0E
	KeyCtrlO          KeyType = 0x0F
	KeyCtrlP          KeyType = 0x10
	KeyCtrlQ          KeyType = 0x11
	KeyCtrlR          KeyType = 0x12
	KeyCtrlS          KeyType = 0x13
	KeyCtrlT          KeyType = 0x14
	KeyCtrlU          KeyType = 0x15
	KeyCtrlV          KeyType = 0x16
	KeyCtrlW          KeyType = 0x17
	KeyCtrlX          KeyType = 0x18
	KeyCtrlY          KeyType = 0x19
	KeyCtrlZ          KeyType = 0x1A
	KeyCtrlLsqBracket KeyType = 0x1B
	KeyCtrl3          KeyType = 0x1B
	KeyEsc            KeyType = 0x1B
	KeyCtrl4          KeyType = 0x1C
	KeyCtrlBackslash  KeyType = 0x1C
	KeyCtrl5          KeyType = 0x1D
	KeyCtrlRsqBracket KeyType = 0x1D
	KeyCtrl6          KeyType = 0x1E
	KeyCtrl7          KeyType = 0x1F
	KeyCtrlSlash      KeyType = 0x1F
	KeyCtrlUnderscore KeyType = 0x1F
	KeySpace          KeyType = 0x20
	KeyBackspace2     KeyType = 0x7F
	KeyCtrl8          KeyType = 0x7F
)

// Aliases
const (
	KeyBackspace KeyType = 0x08 // = KeyCtrlH
	KeyTab       KeyType = 0x09 // = KeyCtrlI
	KeyEnter     KeyType = 0x0D // = KeyCtrlM
)

// This map is populated in init() below.
var stringToKey = map[string]KeyType{}
var keyToString = map[KeyType]string{}

func mapkey(n string, k KeyType) {
	stringToKey[n] = k
	keyToString[k] = n
}

func init() {
	fidx := 1
	for k := KeyF1; k >= KeyF12; k-- {
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
		mapkey(n, KeyType(int(KeyF12)-(i+1)))
	}

	names = []string{
		"Left",
		"Middle",
		"Right",
	}
	for i, n := range names {
		sk := fmt.Sprintf("Mouse%s", n)
		mapkey(sk, KeyType(int(KeyArrowRight)-(i+2)))
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
			mapkey(sk, KeyType(int(KeyCtrlTilde)+i))
		}
	}

	mapkey("BS", KeyBackspace)
	mapkey("Tab", KeyTab)
	mapkey("Enter", KeyEnter)
	mapkey("Esc", KeyEsc)
	mapkey("Space", KeySpace)
	mapkey("BS2", KeyBackspace2)
	mapkey("C-8", KeyCtrl8)
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

// KeyEventToString returns a human-readable name for a key event described
// by the given key type, character, and modifier. This replaces the old
// EventToString that took a termbox.Event directly.
func KeyEventToString(key KeyType, ch rune, mod ModifierKey) (string, error) {
	s := ""
	if key == 0 {
		s = string([]rune{ch})
	} else {
		var ok bool
		s, ok = keyToString[key]
		if !ok {
			return "", errors.Errorf("no such key %d (ch=%c)", key, ch)
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

	if mod == ModAlt {
		return "M-" + s, nil
	}

	return s, nil
}

func ToKey(key string) (k KeyType, modifier ModifierKey, ch rune, err error) {
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
