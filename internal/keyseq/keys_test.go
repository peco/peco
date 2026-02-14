package keyseq

import (
	"testing"
	"unicode/utf8"
)

func TestKeymapStrToKeyValue(t *testing.T) {
	expected := map[string]KeyType{
		"F1":          KeyF1,
		"F2":          KeyF2,
		"F3":          KeyF3,
		"F4":          KeyF4,
		"F5":          KeyF5,
		"F6":          KeyF6,
		"F7":          KeyF7,
		"F8":          KeyF8,
		"F9":          KeyF9,
		"F10":         KeyF10,
		"F11":         KeyF11,
		"F12":         KeyF12,
		"Insert":      KeyInsert,
		"Delete":      KeyDelete,
		"Home":        KeyHome,
		"End":         KeyEnd,
		"Pgup":        KeyPgup,
		"Pgdn":        KeyPgdn,
		"ArrowUp":     KeyArrowUp,
		"ArrowDown":   KeyArrowDown,
		"ArrowLeft":   KeyArrowLeft,
		"ArrowRight":  KeyArrowRight,
		"MouseLeft":   MouseLeft,
		"MouseRight":  MouseRight,
		"MouseMiddle": MouseMiddle,
		"BS":          KeyBackspace,
		"Tab":         KeyTab,
		"Enter":       KeyEnter,
		"Esc":         KeyEsc,
		"Space":       KeySpace,
		"BS2":         KeyBackspace2,
		"C-8":         KeyCtrl8,
		"C-~":         KeyCtrlTilde,
		"C-2":         KeyCtrl2,
		"C-Space":     KeyCtrlSpace,
		"C-a":         KeyCtrlA,
		"C-b":         KeyCtrlB,
		"C-c":         KeyCtrlC,
		"C-d":         KeyCtrlD,
		"C-e":         KeyCtrlE,
		"C-f":         KeyCtrlF,
		"C-g":         KeyCtrlG,
		"C-h":         KeyCtrlH,
		"C-i":         KeyCtrlI,
		"C-j":         KeyCtrlJ,
		"C-k":         KeyCtrlK,
		"C-l":         KeyCtrlL,
		"C-m":         KeyCtrlM,
		"C-n":         KeyCtrlN,
		"C-o":         KeyCtrlO,
		"C-p":         KeyCtrlP,
		"C-q":         KeyCtrlQ,
		"C-r":         KeyCtrlR,
		"C-s":         KeyCtrlS,
		"C-t":         KeyCtrlT,
		"C-u":         KeyCtrlU,
		"C-v":         KeyCtrlV,
		"C-w":         KeyCtrlW,
		"C-x":         KeyCtrlX,
		"C-y":         KeyCtrlY,
		"C-z":         KeyCtrlZ,
		"C-[":         KeyCtrlLsqBracket,
		"C-3":         KeyCtrl3,
		"C-4":         KeyCtrl4,
		"C-\\":        KeyCtrlBackslash,
		"C-5":         KeyCtrl5,
		"C-]":         KeyCtrlRsqBracket,
		"C-6":         KeyCtrl6,
		"C-7":         KeyCtrl7,
		"C-/":         KeyCtrlSlash,
		"C-_":         KeyCtrlUnderscore,
	}

	t.Logf("Checking key name -> actual key value mapping...")
	for n, v := range expected {
		t.Logf("    checking %s...", n)
		e, modifier, _, err := ToKey(n)
		if err != nil {
			t.Errorf("Key name %s not found", n)
		}
		if e != v {
			t.Errorf("Expected '%s' to be '%d', but got '%d'", n, v, stringToKey[n])
		}
		if modifier != 0 {
			t.Errorf("Key name '%s' is not Alt-prefixed", n)
		}
	}
}

func TestKeymapStrToKeyValueWithAlt(t *testing.T) {
	expected := map[string]struct {
		key KeyType
		ch  rune
	}{
		"M-v":         {0, 'v'},
		"M-C-v":       {KeyCtrlV, rune(0)},
		"M-Space":     {KeySpace, rune(0)},
		"M-MouseLeft": {MouseLeft, rune(0)},
	}

	t.Logf("Checking Alt prefixed key name mapping...")
	for n, v := range expected {
		t.Logf("    checking %s...", n)
		k, modifier, ch, err := ToKey(n)
		if err != nil {
			t.Errorf("Failed ToKey: Key name %s", n)
		}
		if modifier != 1 {
			t.Errorf("Key name %s has Alt prefix", n)
		}
		if k != v.key {
			t.Errorf("Expected '%s' to be '%d', but got '%d'", n, v.key, k)
		}
		if ch != v.ch {
			t.Errorf("Expected '%s' to be '%c', but got '%c'", n, v.ch, ch)
		}
	}
}

func TestKeymapStrToKeyValueCh(t *testing.T) {
	expected := []string{
		"q", "w", "e", "r", "t", "y",
		"🙏", "せ", "か", "い", "〠",
	}

	t.Logf("Checking character mapping...")
	for _, n := range expected {
		t.Logf("    checking %s...", n)
		k, modifier, ch, err := ToKey(n)
		if err != nil {
			t.Errorf("Failed ToKey: Key name %s", n)
		}
		if k != 0 {
			t.Errorf("Key name %s is mapped key", n)
		}
		if modifier == 1 {
			t.Errorf("Key name %s has Alt prefix", n)
		}
		r, _ := utf8.DecodeRuneInString(n)
		if ch != r {
			t.Errorf("key name %s cannot convert to rune", n)
		}
	}

}
