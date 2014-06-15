package peco

import (
	"encoding/json"

	"github.com/nsf/termbox-go"
)

var (
	stringToFg = map[string]termbox.Attribute{
		"default": termbox.ColorDefault,
		"black":   termbox.ColorBlack,
		"red":     termbox.ColorRed,
		"green":   termbox.ColorGreen,
		"yellow":  termbox.ColorYellow,
		"blue":    termbox.ColorBlue,
		"magenta": termbox.ColorMagenta,
		"cyan":    termbox.ColorCyan,
		"white":   termbox.ColorWhite,
	}
	stringToBg = map[string]termbox.Attribute{
		"on_default": termbox.ColorDefault,
		"on_black":   termbox.ColorBlack,
		"on_red":     termbox.ColorRed,
		"on_green":   termbox.ColorGreen,
		"on_yellow":  termbox.ColorYellow,
		"on_blue":    termbox.ColorBlue,
		"on_magenta": termbox.ColorMagenta,
		"on_cyan":    termbox.ColorCyan,
		"on_white":   termbox.ColorWhite,
	}
	stringToAttr = map[string]termbox.Attribute{
		"bold":      termbox.AttrBold,
		"underline": termbox.AttrUnderline,
		"blink":     termbox.AttrReverse,
	}
)

type StyleSet struct {
	Basic    Style `json:"Basic"`
	Selected Style `json:"Selected"`
	Query    Style `json:"Query"`
}

func NewStyleSet() StyleSet {
	return StyleSet{
		Basic:    Style{fg: termbox.ColorDefault, bg: termbox.ColorDefault},
		Selected: Style{fg: termbox.ColorDefault | termbox.AttrUnderline, bg: termbox.ColorMagenta},
		Query:    Style{fg: termbox.ColorCyan, bg: termbox.ColorDefault},
	}
}

type Style struct {
	fg termbox.Attribute
	bg termbox.Attribute
}

func (s *Style) UnmarshalJSON(buf []byte) error {
	raw := []string{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return err
	}
	*s = *stringsToStyle(raw)
	return nil
}

func stringsToStyle(raw []string) *Style {
	style := &Style{
		fg: termbox.ColorDefault,
		bg: termbox.ColorDefault,
	}

	for _, s := range raw {
		fg, ok := stringToFg[s]
		if ok {
			style.fg = fg
		}

		bg, ok := stringToBg[s]
		if ok {
			style.bg = bg
		}
	}

	for _, s := range raw {
		attr, ok := stringToAttr[s]
		if ok {
			style.fg |= attr
		}
	}

	return style
}
