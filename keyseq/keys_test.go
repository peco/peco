package keyseq

import (
	"testing"

	"github.com/nsf/termbox-go"
)

func TestKeymapStrToKeyValue(t *testing.T) {
	expected := map[string]termbox.Key{
		"Insert":    termbox.KeyInsert,
		"MouseLeft": termbox.MouseLeft,
		"C-k":       termbox.KeyCtrlK,
		"C-h":       termbox.KeyCtrlH,
		"C-i":       termbox.KeyCtrlI,
		"C-l":       termbox.KeyCtrlL,
		"C-m":       termbox.KeyCtrlM,
		"C-[":       termbox.KeyCtrlLsqBracket,
		"C-\\":      termbox.KeyCtrlBackslash,
		"C-_":       termbox.KeyCtrlUnderscore,
		"C-8":       termbox.KeyCtrl8,
	}

	t.Logf("Checking key name -> actual key value mapping...")
	for n, v := range expected {
		t.Logf("    checking %s...", n)
		// TODO ch isn't being checked
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
