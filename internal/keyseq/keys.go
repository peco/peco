package keyseq

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// KeyType represents a keyboard key. The adapter layer in screen.go maps
// between peco's KeyType and tcell's key constants.
type KeyType uint16

// Function keys
const (
	KeyF1 KeyType = 0xFFFF - iota
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
// There is an intentional gap between KeyArrowRight and MouseLeft
// (MouseLeft = KeyArrowRight - 2, not -1) for historical reasons.
const (
	MouseLeft   KeyType = 0xFFFF - iota - 23 // KeyArrowRight - 2
	MouseMiddle                              // KeyArrowRight - 3
	MouseRight                               // KeyArrowRight - 4
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

// mapkey registers a bidirectional mapping between a key name string and its KeyType constant.
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

	ctrlKeyNames := [][]string{
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
	for i, list := range ctrlKeyNames {
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

// ToKeyList parses a comma-separated key binding string (e.g. "C-x,C-c") into a list of KeySeq values.
func ToKeyList(ksk string) (KeyList, error) {
	list := KeyList{}
	for term := range strings.SplitSeq(ksk, ",") {
		term = strings.TrimSpace(term)

		k, m, ch, err := ToKey(term)
		if err != nil {
			return list, fmt.Errorf("failed to convert '%s': %w", term, err)
		}

		list = append(list, Key{m, k, ch})
	}
	return list, nil
}

// KeyEventToString returns a human-readable name for a key event described
// by the given key type, character, and modifier.
func KeyEventToString(key KeyType, ch rune, mod ModifierKey) (string, error) {
	var s string
	if key == 0 {
		s = string([]rune{ch})
	} else {
		var ok bool
		s, ok = keyToString[key]
		if !ok {
			return "", fmt.Errorf("no such key %d (ch=%c)", key, ch)
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

	if m := mod.String(); m != "" {
		return m + "-" + s, nil
	}

	return s, nil
}

// ToKey parses a single key name string into its KeyType, modifier, and rune components.
func ToKey(key string) (k KeyType, modifier ModifierKey, ch rune, err error) {
	modifier = ModNone

	// Try full string first. This handles legacy key names like "C-a",
	// "C-v", "Home", "ArrowLeft", etc. that are registered in stringToKey.
	if k, ok := stringToKey[key]; ok {
		return k, modifier, 0, nil
	}

	// Parse modifier prefixes (C-, S-, M-) iteratively.
	// After each prefix is stripped, try the remainder as a key name.
	for {
		switch {
		case strings.HasPrefix(key, "C-"):
			modifier |= ModCtrl
			key = key[2:]
		case strings.HasPrefix(key, "S-"):
			modifier |= ModShift
			key = key[2:]
		case strings.HasPrefix(key, "M-"):
			modifier |= ModAlt
			key = key[2:]
		default:
			goto done
		}

		// After stripping a prefix, try as a registered key name.
		// This handles e.g. "M-C-v" → strip M-, then "C-v" is found.
		if k, ok := stringToKey[key]; ok {
			return k, modifier, 0, nil
		}

		// Single ASCII char after modifier(s) → treat as rune
		if len(key) == 1 {
			return 0, modifier, rune(key[0]), nil
		}
	}

done:
	// Try as a single rune (handles multi-byte chars like "せ")
	ch, _ = utf8.DecodeRuneInString(key)
	if ch != utf8.RuneError {
		return 0, modifier, ch, nil
	}

	return 0, modifier, 0, fmt.Errorf("no such key %s", key)
}
