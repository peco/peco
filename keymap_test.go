package peco

import (
	"encoding/json"
	"testing"

	"github.com/nsf/termbox-go"
)

func TestReadRC(t *testing.T) {
	txt := `
{
  "Keymap": {
   "C-k": "peco.Exit"
  }
}
`
	cfg := NewConfig()
	if err := json.Unmarshal([]byte(txt), cfg); err != nil {
		t.Fatalf("Error unmarshaling json: %s", err)
	}
	t.Logf("%#q", cfg)
}

func TestKeymapStrToKeyValue(t *testing.T) {
	expected := map[string]termbox.Key{
		"Insert": termbox.KeyInsert,
		"MouseLeft": termbox.MouseLeft,
		"C-k": termbox.KeyCtrlK,
		"C-h": termbox.KeyCtrlH,
		"C-i": termbox.KeyCtrlI,
		"C-l": termbox.KeyCtrlL,
		"C-m": termbox.KeyCtrlM,
		"C-[": termbox.KeyCtrlLsqBracket,
		"C-\\": termbox.KeyCtrlBackslash,
		"C-_": termbox.KeyCtrlUnderscore,
		"C-8": termbox.KeyCtrl8,
	}

t.Logf("%#q", stringToKey)

	t.Logf("Checking key name -> actual key value mapping...")
	for n, v := range expected {
		t.Logf("    checking %s...", n)
		e, ok := stringToKey[n]
		if !ok {
			t.Errorf("Key name %s not found", n)
		}
		if e != v {
			t.Errorf("Expected '%s' to be '%d', but got '%d'", n, v, stringToKey[n])
		}
	}
}
