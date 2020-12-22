package peco

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
)

// Color represents a color. In termbox terms, colors are part of attributes
// so there is really no point in having a different type, but we do this
// so that the API is easier to understand
type Color = termbox.Attribute

const (
	ColorDefault      = termbox.ColorDefault
	ColorBlack        = termbox.ColorBlack
	ColorRed          = termbox.ColorRed
	ColorGreen        = termbox.ColorGreen
	ColorYellow       = termbox.ColorYellow
	ColorBlue         = termbox.ColorBlue
	ColorMagenta      = termbox.ColorMagenta
	ColorCyan         = termbox.ColorCyan
	ColorWhite        = termbox.ColorWhite
	ColorDarkGray     = termbox.ColorDarkGray
	ColorLightRed     = termbox.ColorLightRed
	ColorLightGreen   = termbox.ColorLightGreen
	ColorLightYellow  = termbox.ColorLightYellow
	ColorLightBlue    = termbox.ColorLightBlue
	ColorLightMagenta = termbox.ColorLightMagenta
	ColorLightCyan    = termbox.ColorLightCyan
	ColorLightGray    = termbox.ColorLightGray
)

// Style represents the set of styles to be applied when printing to the
// terminal.
type Style struct {
	// TODO: separate tcell/termbox versions so we can hide termbox from the
	// rest of the peco codebase
	bg    termbox.Attribute
	fg    termbox.Attribute
	attrs termbox.Attribute
}

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

var stylePool = sync.Pool{
	New: func() interface{} { return NewStyle() },
}

func NewStyle() *Style {
	return &Style{}
}

func FetchStyle() *Style {
	return stylePool.Get().(*Style)
}

func (s *Style) Release() {
	s.Reset()
	stylePool.Put(s)
}

func (s *Style) Reset() *Style {
	s.bg = 0
	s.fg = 0
	s.attrs = 0
	return s
}

func (s *Style) Foreground(c Color) *Style {
	s.fg = c
	return s
}

func (s *Style) Background(c Color) *Style {
	s.bg = c
	return s
}

func (s *Style) setAttr(attr termbox.Attribute, on bool) *Style {
	if on {
		s.attrs |= attr
	} else {
		s.attrs &^= attr
	}
	return s
}

func (s *Style) Bold(v bool) *Style {
	return s.setAttr(termbox.AttrBold, v)
}

func (s *Style) Blink(v bool) *Style {
	return s.setAttr(termbox.AttrBlink, v)
}

func (s *Style) Hidden(v bool) *Style {
	return s.setAttr(termbox.AttrHidden, v)
}

func (s *Style) Dim(v bool) *Style {
	return s.setAttr(termbox.AttrDim, v)
}

func (s *Style) Underline(v bool) *Style {
	return s.setAttr(termbox.AttrUnderline, v)
}

func (s *Style) Cursive(v bool) *Style {
	return s.setAttr(termbox.AttrCursive, v)
}

func (s *Style) Reverse(v bool) *Style {
	return s.setAttr(termbox.AttrReverse, v)
}

// UnmarshalJSON satisfies json.RawMessage.
func (s *Style) UnmarshalJSON(buf []byte) error {
	raw := []string{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return errors.Wrapf(err, "failed to unmarshal Style")
	}
	return s.FromStrings(raw...)
}

func (style *Style) FromStrings(raw ...string) error {
	style.Reset()

	for _, s := range raw {
		fg, ok := stringToFg[s]
		if ok {
			style.Foreground(fg)
		} else {
			if fg, err := strconv.ParseUint(s, 10, 8); err == nil {
				style.Foreground(termbox.Attribute(fg + 1))
			}
		}

		bg, ok := stringToBg[s]
		if ok {
			style.Background(bg)
		} else {
			if strings.HasPrefix(s, "on_") {
				if bg, err := strconv.ParseUint(s[3:], 10, 8); err == nil {
					style.Background(termbox.Attribute(bg + 1))
				}
			}
		}
	}

	for _, s := range raw {
		if fgAttr, ok := stringToFgAttr[s]; ok {
			style.setAttr(fgAttr, true)
		} else if bgAttr, ok := stringToBgAttr[s]; ok {
			style.setAttr(bgAttr, true)
		}
	}

	return nil
}

func (s *Style) Clone() *Style {
	cloned := NewStyle()
	cloned.fg = s.fg
	cloned.bg = s.bg
	cloned.attrs = s.attrs
	return cloned
}
