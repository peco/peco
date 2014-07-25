package peco

import (
	"github.com/nsf/termbox-go"
	"testing"
)

func TestMergeAttribute(t *testing.T) {
	colors := stringToFg

	// merge colors
	tests := [][]string{
		{"red", "green", "yellow"},
		{"red", "blue", "magenta"},
		{"green", "blue", "cyan"},
		{"yellow", "blue", "white"},
		{"magenta", "green", "white"},
		{"cyan", "red", "white"},
		{"yellow", "magenta", "white"},
		{"magenta", "cyan", "white"},
		{"cyan", "yellow", "white"},
	}

	for _, c := range tests {
		if m := mergeAttribute(colors[c[0]], colors[c[1]]); m != colors[c[2]] {
			t.Errorf("(%s + %s) expected %s, got %s", c[0], c[1], colors[c[2]], m)
		}
	}

	// merge with white
	for _, c := range colors {
		if m := mergeAttribute(c, colors["white"]); m != colors["white"] {
			t.Errorf("expected white(%s), got %s", colors["white"], m)
		}
	}

	// merge attributes
	if m := mergeAttribute(termbox.AttrBold|colors["red"], termbox.AttrUnderline|colors["cyan"]); m != termbox.AttrBold|termbox.AttrUnderline|colors["white"] {
		t.Errorf("expected %s, got %s", termbox.AttrBold|termbox.AttrUnderline|colors["white"], m)
	}

}
