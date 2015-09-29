package keyseq

import (
	"testing"
	"unicode/utf8"

	"github.com/nsf/termbox-go"
)

func TestKeymapStrToKeyValue(t *testing.T) {
	expected := map[string]termbox.Key{
		"F1":          termbox.KeyF1,
		"F2":          termbox.KeyF2,
		"F3":          termbox.KeyF3,
		"F4":          termbox.KeyF4,
		"F5":          termbox.KeyF5,
		"F6":          termbox.KeyF6,
		"F7":          termbox.KeyF7,
		"F8":          termbox.KeyF8,
		"F9":          termbox.KeyF9,
		"F10":         termbox.KeyF10,
		"F11":         termbox.KeyF11,
		"F12":         termbox.KeyF12,
		"Insert":      termbox.KeyInsert,
		"Delete":      termbox.KeyDelete,
		"Home":        termbox.KeyHome,
		"End":         termbox.KeyEnd,
		"Pgup":        termbox.KeyPgup,
		"Pgdn":        termbox.KeyPgdn,
		"ArrowUp":     termbox.KeyArrowUp,
		"ArrowDown":   termbox.KeyArrowDown,
		"ArrowLeft":   termbox.KeyArrowLeft,
		"ArrowRight":  termbox.KeyArrowRight,
		"MouseLeft":   termbox.MouseLeft,
		"MouseRight":  termbox.MouseRight,
		"MouseMiddle": termbox.MouseMiddle,
		"BS":          termbox.KeyBackspace,
		"Tab":         termbox.KeyTab,
		"Enter":       termbox.KeyEnter,
		"Esc":         termbox.KeyEsc,
		"Space":       termbox.KeySpace,
		"BS2":         termbox.KeyBackspace2,
		"C-8":         termbox.KeyCtrl8,
		"C-~":         termbox.KeyCtrlTilde,
		"C-2":         termbox.KeyCtrl2,
		"C-Space":     termbox.KeyCtrlSpace,
		"C-a":         termbox.KeyCtrlA,
		"C-b":         termbox.KeyCtrlB,
		"C-c":         termbox.KeyCtrlC,
		"C-d":         termbox.KeyCtrlD,
		"C-e":         termbox.KeyCtrlE,
		"C-f":         termbox.KeyCtrlF,
		"C-g":         termbox.KeyCtrlG,
		"C-h":         termbox.KeyCtrlH,
		"C-i":         termbox.KeyCtrlI,
		"C-j":         termbox.KeyCtrlJ,
		"C-k":         termbox.KeyCtrlK,
		"C-l":         termbox.KeyCtrlL,
		"C-m":         termbox.KeyCtrlM,
		"C-n":         termbox.KeyCtrlN,
		"C-o":         termbox.KeyCtrlO,
		"C-p":         termbox.KeyCtrlP,
		"C-q":         termbox.KeyCtrlQ,
		"C-r":         termbox.KeyCtrlR,
		"C-s":         termbox.KeyCtrlS,
		"C-t":         termbox.KeyCtrlT,
		"C-u":         termbox.KeyCtrlU,
		"C-v":         termbox.KeyCtrlV,
		"C-w":         termbox.KeyCtrlW,
		"C-x":         termbox.KeyCtrlX,
		"C-y":         termbox.KeyCtrlY,
		"C-z":         termbox.KeyCtrlZ,
		"C-[":         termbox.KeyCtrlLsqBracket,
		"C-3":         termbox.KeyCtrl3,
		"C-4":         termbox.KeyCtrl4,
		"C-\\":        termbox.KeyCtrlBackslash,
		"C-5":         termbox.KeyCtrl5,
		"C-]":         termbox.KeyCtrlRsqBracket,
		"C-6":         termbox.KeyCtrl6,
		"C-7":         termbox.KeyCtrl7,
		"C-/":         termbox.KeyCtrlSlash,
		"C-_":         termbox.KeyCtrlUnderscore,
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
		key termbox.Key
		ch  rune
	}{
		"M-v":         {0, 'v'},
		"M-C-v":       {termbox.KeyCtrlV, rune(0)},
		"M-Space":     {termbox.KeySpace, rune(0)},
		"M-MouseLeft": {termbox.MouseLeft, rune(0)},
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
