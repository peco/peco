package peco

import (
	"testing"

	"github.com/nsf/termbox-go"
)

type stringsToStyleTest struct {
	strings []string
	style   *Style
}

func TestStringsToStyle(t *testing.T) {
	tests := []stringsToStyleTest{
		stringsToStyleTest{
			strings: []string{"on_default", "default"},
			style:   &Style{fg: termbox.ColorDefault, bg: termbox.ColorDefault},
		},
		stringsToStyleTest{
			strings: []string{"bold", "on_blue", "yellow"},
			style:   &Style{fg: termbox.ColorYellow | termbox.AttrBold, bg: termbox.ColorBlue},
		},
		stringsToStyleTest{
			strings: []string{"underline", "on_cyan", "black"},
			style:   &Style{fg: termbox.ColorBlack | termbox.AttrUnderline, bg: termbox.ColorCyan},
		},
		stringsToStyleTest{
			strings: []string{"blink", "on_red", "white"},
			style:   &Style{fg: termbox.ColorWhite | termbox.AttrReverse, bg: termbox.ColorRed},
		},
	}

	t.Logf("Checking strings -> color mapping...")
	for _, test := range tests {
		t.Logf("    checking %s...", test.strings)
		if a := stringsToStyle(test.strings); *a != *test.style {
			t.Errorf("Expected '%s' to be '%#v', but got '%#v'", test.strings, test.style, a)
		}
	}
}
