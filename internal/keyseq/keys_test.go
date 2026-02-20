package keyseq

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
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
		require.NoError(t, err, "Key name %s not found", n)
		require.Equal(t, v, e, "Expected '%s' to be '%d', but got '%d'", n, v, stringToKey[n])
		require.Equal(t, ModifierKey(0), modifier, "Key name '%s' is not Alt-prefixed", n)
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
		require.NoError(t, err, "Failed ToKey: Key name %s", n)
		require.Equal(t, ModifierKey(1), modifier, "Key name %s has Alt prefix", n)
		require.Equal(t, v.key, k, "Expected '%s' to be '%d', but got '%d'", n, v.key, k)
		require.Equal(t, v.ch, ch, "Expected '%s' to be '%c', but got '%c'", n, v.ch, ch)
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
		require.NoError(t, err, "Failed ToKey: Key name %s", n)
		require.Equal(t, KeyType(0), k, "Key name %s is mapped key", n)
		require.NotEqual(t, ModifierKey(1), modifier, "Key name %s has Alt prefix", n)
		r, _ := utf8.DecodeRuneInString(n)
		require.Equal(t, r, ch, "key name %s cannot convert to rune", n)
	}
}

func TestKeymapStrToKeyValueWithCtrl(t *testing.T) {
	tests := []struct {
		name string
		key  KeyType
	}{
		{"C-ArrowLeft", KeyArrowLeft},
		{"C-ArrowRight", KeyArrowRight},
		{"C-ArrowUp", KeyArrowUp},
		{"C-ArrowDown", KeyArrowDown},
		{"C-Home", KeyHome},
		{"C-End", KeyEnd},
		{"C-Delete", KeyDelete},
		{"C-Insert", KeyInsert},
		{"C-Pgup", KeyPgup},
		{"C-Pgdn", KeyPgdn},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, modifier, ch, err := ToKey(tc.name)
			require.NoError(t, err)
			require.Equal(t, tc.key, k)
			require.Equal(t, ModCtrl, modifier)
			require.Equal(t, rune(0), ch)
		})
	}
}

func TestKeymapStrToKeyValueWithShift(t *testing.T) {
	tests := []struct {
		name string
		key  KeyType
	}{
		{"S-ArrowUp", KeyArrowUp},
		{"S-ArrowDown", KeyArrowDown},
		{"S-ArrowLeft", KeyArrowLeft},
		{"S-ArrowRight", KeyArrowRight},
		{"S-Home", KeyHome},
		{"S-End", KeyEnd},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, modifier, ch, err := ToKey(tc.name)
			require.NoError(t, err)
			require.Equal(t, tc.key, k)
			require.Equal(t, ModShift, modifier)
			require.Equal(t, rune(0), ch)
		})
	}
}

func TestKeymapStrToKeyValueWithCombinedModifiers(t *testing.T) {
	tests := []struct {
		name     string
		key      KeyType
		modifier ModifierKey
	}{
		{"C-M-ArrowLeft", KeyArrowLeft, ModCtrl | ModAlt},
		{"M-C-ArrowLeft", KeyArrowLeft, ModCtrl | ModAlt},
		{"C-S-Delete", KeyDelete, ModCtrl | ModShift},
		{"C-S-M-Home", KeyHome, ModCtrl | ModShift | ModAlt},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, modifier, ch, err := ToKey(tc.name)
			require.NoError(t, err)
			require.Equal(t, tc.key, k)
			require.Equal(t, tc.modifier, modifier)
			require.Equal(t, rune(0), ch)
		})
	}
}

func TestModifierKeyString(t *testing.T) {
	tests := []struct {
		mod      ModifierKey
		expected string
	}{
		{ModNone, ""},
		{ModAlt, "M"},
		{ModCtrl, "C"},
		{ModShift, "S"},
		{ModCtrl | ModAlt, "C-M"},
		{ModCtrl | ModShift, "C-S"},
		{ModShift | ModAlt, "S-M"},
		{ModCtrl | ModShift | ModAlt, "C-S-M"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.mod.String())
		})
	}
}

func TestKeyEventToStringWithModifiers(t *testing.T) {
	tests := []struct {
		name     string
		key      KeyType
		ch       rune
		mod      ModifierKey
		expected string
	}{
		{"Ctrl+Left", KeyArrowLeft, 0, ModCtrl, "C-<"},
		{"Shift+Right", KeyArrowRight, 0, ModShift, "S->"},
		{"Ctrl+Alt+Delete", KeyDelete, 0, ModCtrl | ModAlt, "C-M-Delete"},
		{"Alt+char", 0, 'v', ModAlt, "M-v"},
		{"no modifier", KeyHome, 0, ModNone, "Home"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := KeyEventToString(tc.key, tc.ch, tc.mod)
			require.NoError(t, err)
			require.Equal(t, tc.expected, s)
		})
	}
}
