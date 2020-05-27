package ui

import (
	"encoding/json"

	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
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
	stringToFgAttr = map[string]termbox.Attribute{
		"bold":      termbox.AttrBold,
		"underline": termbox.AttrUnderline,
		"reverse":   termbox.AttrReverse,
	}
	stringToBgAttr = map[string]termbox.Attribute{
		"on_bold": termbox.AttrBold,
	}
)

// NewStyleSet creates a new StyleSet struct
func NewStyleSet() *StyleSet {
	ss := &StyleSet{
		Basic:          NewStyle(termbox.ColorDefault, termbox.ColorDefault),
		Query:          NewStyle(termbox.ColorDefault, termbox.ColorDefault),
		Matched:        NewStyle(termbox.ColorDefault, termbox.ColorDefault),
		SavedSelection: NewStyle(termbox.ColorDefault, termbox.ColorDefault),
		Selected:       NewStyle(termbox.ColorDefault, termbox.ColorDefault),
	}
	ss.applyDefaults()
	return ss
}

func (ss *StyleSet) applyDefaults() {
	ss.Basic.fg = termbox.ColorDefault
	ss.Basic.bg = termbox.ColorDefault
	ss.Query.fg = termbox.ColorDefault
	ss.Query.bg = termbox.ColorDefault
	ss.Matched.fg = termbox.ColorCyan
	ss.Matched.bg = termbox.ColorDefault
	ss.SavedSelection.fg = termbox.ColorBlack | termbox.AttrBold
	ss.SavedSelection.bg = termbox.ColorCyan
	ss.Selected.fg = termbox.ColorDefault | termbox.AttrUnderline
	ss.Selected.bg = termbox.ColorMagenta
}

func (s *Style) Foreground() termbox.Attribute {
	return s.fg
}

func NewStyle(fg, bg termbox.Attribute) *Style {
	return &Style{fg: fg, bg: bg}
}

func (s *Style) Background() termbox.Attribute {
	return s.bg
}

// UnmarshalJSON decodes the JSON representation and assembles
// the proper Style object from a list of strings
func (s *Style) UnmarshalJSON(buf []byte) error {
	raw := []string{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return errors.Wrapf(err, "failed to unmarshal Style")
	}

	s.fg = termbox.ColorDefault
	s.bg = termbox.ColorDefault

	for _, v := range raw {
		fg, ok := stringToFg[v]
		if ok {
			s.fg |= fg
			continue
		}

		bg, ok := stringToBg[v]
		if ok {
			s.bg |= bg
			continue
		}

		if fgAttr, ok := stringToFgAttr[v]; ok {
			s.fg |= fgAttr
			continue
		}

		if bgAttr, ok := stringToBgAttr[v]; ok {
			s.bg |= bgAttr
			continue
		}
	}

	return nil
}
