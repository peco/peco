package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// StyleSet holds styles for various sections
type StyleSet struct {
	Basic          Style `json:"Basic" yaml:"Basic"`
	SavedSelection Style `json:"SavedSelection" yaml:"SavedSelection"`
	Selected       Style `json:"Selected" yaml:"Selected"`
	Query          Style `json:"Query" yaml:"Query"`
	QueryCursor    Style `json:"QueryCursor" yaml:"QueryCursor"`
	Matched        Style `json:"Matched" yaml:"Matched"`
	Prompt         Style `json:"Prompt" yaml:"Prompt"`
	Context        Style `json:"Context" yaml:"Context"`
}

// Attribute represents terminal display attributes such as colors
// and text styling (bold, underline, reverse). It is a uint32 bitfield:
//
//	Bits 0-8:   Palette color index (0=default, 1-256 for 256-color palette)
//	Bits 0-23:  RGB color value (when AttrTrueColor flag is set)
//	Bit 24:     AttrTrueColor flag — distinguishes true color from palette
//	Bit 25:     AttrBold
//	Bit 26:     AttrUnderline
//	Bit 27:     AttrReverse
//	Bits 28-31: Reserved
type Attribute uint32

// Named palette color constants (values 0-8).
const (
	ColorDefault Attribute = 0x0000
	ColorBlack   Attribute = 0x0001
	ColorRed     Attribute = 0x0002
	ColorGreen   Attribute = 0x0003
	ColorYellow  Attribute = 0x0004
	ColorBlue    Attribute = 0x0005
	ColorMagenta Attribute = 0x0006
	ColorCyan    Attribute = 0x0007
	ColorWhite   Attribute = 0x0008
)

const (
	AttrTrueColor Attribute = 0x01000000
	AttrBold      Attribute = 0x02000000
	AttrUnderline Attribute = 0x04000000
	AttrReverse   Attribute = 0x08000000
)

// Style describes display attributes for foreground and background.
type Style struct {
	Fg Attribute
	Bg Attribute
}

var (
	StringToFg = map[string]Attribute{
		"default": ColorDefault,
		"black":   ColorBlack,
		"red":     ColorRed,
		"green":   ColorGreen,
		"yellow":  ColorYellow,
		"blue":    ColorBlue,
		"magenta": ColorMagenta,
		"cyan":    ColorCyan,
		"white":   ColorWhite,
	}
	StringToBg = map[string]Attribute{
		"on_default": ColorDefault,
		"on_black":   ColorBlack,
		"on_red":     ColorRed,
		"on_green":   ColorGreen,
		"on_yellow":  ColorYellow,
		"on_blue":    ColorBlue,
		"on_magenta": ColorMagenta,
		"on_cyan":    ColorCyan,
		"on_white":   ColorWhite,
	}
	StringToFgAttr = map[string]Attribute{
		"bold":      AttrBold,
		"underline": AttrUnderline,
		"reverse":   AttrReverse,
	}
	StringToBgAttr = map[string]Attribute{
		"on_bold": AttrBold,
	}
)

// NewStyleSet creates a new StyleSet struct
func NewStyleSet() *StyleSet {
	ss := &StyleSet{}
	ss.Init()
	return ss
}

// Init initializes the StyleSet with default foreground and background colors
// for each UI element (basic, query, matched, selected, prompt, context, etc.).
func (ss *StyleSet) Init() {
	ss.Basic.Fg = ColorDefault
	ss.Basic.Bg = ColorDefault
	ss.Query.Fg = ColorDefault
	ss.Query.Bg = ColorDefault
	ss.Matched.Fg = ColorCyan
	ss.Matched.Bg = ColorDefault
	ss.SavedSelection.Fg = ColorBlack | AttrBold
	ss.SavedSelection.Bg = ColorCyan
	ss.Selected.Fg = ColorDefault | AttrUnderline
	ss.Selected.Bg = ColorMagenta
	ss.Prompt.Fg = ColorDefault
	ss.Prompt.Bg = ColorDefault
	ss.Context.Fg = ColorDefault | AttrBold
	ss.Context.Bg = ColorDefault
}

// UnmarshalJSON satisfies json.RawMessage.
func (s *Style) UnmarshalJSON(buf []byte) error {
	raw := []string{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal Style: %w", err)
	}
	return StringsToStyle(s, raw)
}

// UnmarshalYAML decodes a YAML array of strings into a Style.
func (s *Style) UnmarshalYAML(unmarshal func(any) error) error {
	var raw []string
	if err := unmarshal(&raw); err != nil {
		return fmt.Errorf("failed to unmarshal Style from YAML: %w", err)
	}
	return StringsToStyle(s, raw)
}

// StringsToStyle parses an array of color and attribute strings (e.g. "red",
// "on_blue", "bold", "#ff00ff") into a Style's foreground and background Attributes.
func StringsToStyle(style *Style, raw []string) error {
	style.Fg = ColorDefault
	style.Bg = ColorDefault

	for _, s := range raw {
		fg, ok := StringToFg[s]
		if ok {
			style.Fg = fg
		} else if strings.HasPrefix(s, "#") && len(s) == 7 {
			if rgb, err := strconv.ParseUint(s[1:], 16, 32); err == nil {
				style.Fg = Attribute(rgb) | AttrTrueColor
			}
		} else {
			if fg, err := strconv.ParseUint(s, 10, 8); err == nil {
				style.Fg = Attribute(fg + 1)
			}
		}

		bg, ok := StringToBg[s]
		if ok {
			style.Bg = bg
		} else if strings.HasPrefix(s, "on_#") && len(s) == 10 {
			if rgb, err := strconv.ParseUint(s[4:], 16, 32); err == nil {
				style.Bg = Attribute(rgb) | AttrTrueColor
			}
		} else {
			if strings.HasPrefix(s, "on_") {
				if bg, err := strconv.ParseUint(s[3:], 10, 8); err == nil {
					style.Bg = Attribute(bg + 1)
				}
			}
		}
	}

	for _, s := range raw {
		if fgAttr, ok := StringToFgAttr[s]; ok {
			style.Fg |= fgAttr
		}

		if bgAttr, ok := StringToBgAttr[s]; ok {
			style.Bg |= bgAttr
		}
	}

	return nil
}
