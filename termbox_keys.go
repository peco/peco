// +build !tcell

package peco

import (
	"fmt"

	"github.com/pkg/errors"
)

type KeyDB struct {
	s2k map[string]KeyCode
	k2s map[KeyCode]string
}

// gKeyDB is the global key db
var gKeyDB = &KeyDB{
	s2k: make(map[string]KeyCode),
	k2s: make(map[KeyCode]string),
}

func (db *KeyDB) Map(n string, k KeyCode) {
	// key->string can only have one mapping
	if _, ok := db.k2s[k]; !ok {
		db.k2s[k] = n
	}

	// Multiple string representation can be mapped to the same key
	db.s2k[n] = k
}

func init() {
	fidx := 1
	for k := KeyF1; k >= KeyF12; k-- {
		sk := fmt.Sprintf("F%d", fidx)
		gKeyDB.Map(sk, k)
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
		gKeyDB.Map(n, KeyCode(int(KeyF12)-(i+1)))
	}

	names = []string{
		"Left",
		"Middle",
		"Right",
	}
	for i, n := range names {
		sk := fmt.Sprintf("Mouse%s", n)
		gKeyDB.Map(sk, KeyCode(int(KeyArrowRight)-(i+2)))
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
			gKeyDB.Map(sk, KeyCode(int(KeyCtrlTilde)+i))
		}
	}

	gKeyDB.Map("BS", KeyBackspace)
	gKeyDB.Map("Tab", KeyTab)
	gKeyDB.Map("Enter", KeyEnter)
	gKeyDB.Map("Esc", KeyEsc)
	gKeyDB.Map("Space", KeySpace)
	gKeyDB.Map("BS2", KeyBackspace2)
	gKeyDB.Map("C-8", KeyCtrl8)
}

func (db *KeyDB) Format(ev Event) (string, error) {
	s := ""
	if ev.KeyCode() == 0 {
		s = string([]rune{ev.Rune()})
	} else {
		var ok bool
		s, ok = db.k2s[ev.KeyCode()]
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

	if ev.HasModifier(ModAlt) {
		return "M-" + s, nil
	}

	return s, nil
}
