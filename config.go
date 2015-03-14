package peco

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nsf/termbox-go"
)

// DefaultCustomFilterBufferThreshold is the default value
// for BufferThreshold setting on CustomFilters. 
const DefaultCustomFilterBufferThreshold = 100

var homedirFunc = homedir

// Config holds all the data that can be configured in the
// external configuran file
type Config struct {
	Action map[string][]string `json:"Action"`
	// Keymap used to be directly responsible for dispatching
	// events against user input, but since then this has changed
	// into something that just records the user's config input
	Keymap          map[string]string `json:"Keymap"`
	Matcher         string            `json:"Matcher"`        // Deprecated.
	InitialMatcher  string            `json:"InitialMatcher"` // Use this instead of Matcher
	InitialFilter   string            `json:"InitialFilter"`
	Style           *StyleSet         `json:"Style"`
	Prompt          string            `json:"Prompt"`
	Layout          string            `json:"Layout"`
	CustomMatcher   map[string][]string
	CustomFilter    map[string]CustomFilterConfig
	StickySelection bool
}

type CustomFilterConfig struct {
	// Cmd is the name of the command to invoke
	Cmd             string

	// TODO: need to check if how we use this is correct
	Args            []string

	// BufferThreshold defines how many lines peco buffers before
	// invoking the external command. If this value is big, we
	// will execute the external command fewer times, but the
	// results will not be generated for longer periods of time.
	// If this value is small, we will execute the external command
	// more often, but you pay the penalty of invoking that command
	// more times.
	BufferThreshold int
}

// NewConfig creates a new Config
func NewConfig() *Config {
	return &Config{
		Keymap:         make(map[string]string),
		InitialMatcher: IgnoreCaseMatch,
		Style:          NewStyleSet(),
		Prompt:         "QUERY>",
		Layout:         "top-down",
	}
}

// ReadFilename reads the config from the given file, and
// does the appropriate processing, if any
func (c *Config) ReadFilename(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(c)
	if err != nil {
		return err
	}

	if !IsValidLayoutType(LayoutType(c.Layout)) {
		return fmt.Errorf("invalid layout type: %s", c.Layout)
	}

	if len(c.CustomMatcher) > 0 {
		fmt.Fprintf(os.Stderr, "'CustomMatcher' is deprecated. Use CustomFilter instead\n")

		for n, cfg := range c.CustomMatcher {
			if _, ok := c.CustomFilter[n]; ok {
				return fmt.Errorf("CustomFilter '%s' already exists. Refusing to overwrite with deprecated CustomMatcher config", n)
			}

			c.CustomFilter[n] = CustomFilterConfig{
				Cmd:             cfg[0],
				Args:            cfg[1:],
				BufferThreshold: DefaultCustomFilterBufferThreshold,
			}
		}
	}

	return nil
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

// StyleSet holds styles for various sections
type StyleSet struct {
	Basic          Style `json:"Basic"`
	SavedSelection Style `json:"SavedSelection"`
	Selected       Style `json:"Selected"`
	Query          Style `json:"Query"`
	Matched        Style `json:"Matched"`
}

// NewStyleSet creates a new StyleSet struct
func NewStyleSet() *StyleSet {
	return &StyleSet{
		Basic:          Style{fg: termbox.ColorDefault, bg: termbox.ColorDefault},
		Query:          Style{fg: termbox.ColorDefault, bg: termbox.ColorDefault},
		Matched:        Style{fg: termbox.ColorCyan, bg: termbox.ColorDefault},
		SavedSelection: Style{fg: termbox.ColorBlack | termbox.AttrBold, bg: termbox.ColorCyan},
		Selected:       Style{fg: termbox.ColorDefault | termbox.AttrUnderline, bg: termbox.ColorMagenta},
	}
}

// Style describes termbox styles
type Style struct {
	fg termbox.Attribute
	bg termbox.Attribute
}

// UnmarshalJSON satisfies json.RawMessage.
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
		if fgAttr, ok := stringToFgAttr[s]; ok {
			style.fg |= fgAttr
		}

		if bgAttr, ok := stringToBgAttr[s]; ok {
			style.bg |= bgAttr
		}
	}

	return style
}

// This is a variable because we want to change its behavior
// when we run tests.
var locateRcfileInFunc = locateRcfileIn

func locateRcfileIn(dir string) (string, error) {
	const basename = "config.json"
	file := filepath.Join(dir, basename)
	if _, err := os.Stat(file); err != nil {
		return "", err
	}
	return file, nil
}

// LocateRcfile attempts to find the config file in various locations
func LocateRcfile() (string, error) {
	// http://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html
	//
	// Try in this order:
	//	  $XDG_CONFIG_HOME/peco/config.json
	//    $XDG_CONFIG_DIR/peco/config.json (where XDG_CONFIG_DIR is listed in $XDG_CONFIG_DIRS)
	//	  ~/.peco/config.json

	home, uErr := homedirFunc()

	// Try dir supplied via env var
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		file, err := locateRcfileInFunc(filepath.Join(dir, "peco"))
		if err == nil {
			return file, nil
		}
	} else if uErr == nil { // silently ignore failure for homedir()
		// Try "default" XDG location, is user is available
		file, err := locateRcfileInFunc(filepath.Join(home, ".config", "peco"))
		if err == nil {
			return file, nil
		}
	}

	// this standard does not take into consideration windows (duh)
	// while the spec says use ":" as the separator, Go provides us
	// with filepath.ListSeparator, so use it
	if dirs := os.Getenv("XDG_CONFIG_DIRS"); dirs != "" {
		for _, dir := range strings.Split(dirs, fmt.Sprintf("%c", filepath.ListSeparator)) {
			file, err := locateRcfileInFunc(filepath.Join(dir, "peco"))
			if err == nil {
				return file, nil
			}
		}
	}

	if uErr == nil { // silently ignore failure for homedir()
		file, err := locateRcfileInFunc(filepath.Join(home, ".peco"))
		if err == nil {
			return file, nil
		}
	}

	return "", fmt.Errorf("error: Config file not found")
}
