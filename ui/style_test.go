package ui_test

import (
	"encoding/json"
	"testing"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/ui"
	"github.com/stretchr/testify/assert"
)

func TestStyle(t *testing.T) {
	tests := []struct {
		Source string
		Style  *ui.Style
	}{
		{
			Source: `["black","on_cyan","on_bold"]`,
			Style:  ui.NewStyle(termbox.ColorBlack, termbox.ColorCyan|termbox.AttrBold),
		},
		{
			Source: `["on_default", "default"]`,
			Style:  ui.NewStyle(termbox.ColorDefault, termbox.ColorDefault),
		},
		{
			Source: `["bold", "on_blue", "yellow"]`,
			Style:  ui.NewStyle(termbox.ColorYellow|termbox.AttrBold, termbox.ColorBlue),
		},
		{
			Source: `["underline", "on_cyan", "black"]`,
			Style:  ui.NewStyle(termbox.ColorBlack|termbox.AttrUnderline, termbox.ColorCyan),
		},
		{
			Source: `["reverse", "on_red", "white"]`,
			Style:  ui.NewStyle(termbox.ColorWhite|termbox.AttrReverse, termbox.ColorRed),
		},
		{
			Source: `["on_bold", "on_magenta", "green"]`,
			Style:  ui.NewStyle(termbox.ColorGreen, termbox.ColorMagenta|termbox.AttrBold),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.Source, func(t *testing.T) {
			var s ui.Style
			if !assert.NoError(t, json.Unmarshal([]byte(test.Source), &s), `json.Unmarshal should succeed`) {
				return
			}

			if !assert.Equal(t, &s, test.Style, `style should match`) {
				return
			}
		})
	}
}
