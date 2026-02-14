package peco

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/internal/util"
	"github.com/pkg/errors"
)

var homedirFunc = util.Homedir

// Init initializes the Config with default values
func (c *Config) Init() error {
	c.Keymap = make(map[string]string)
	c.InitialMatcher = IgnoreCaseMatch
	c.Style.Init()
	c.Prompt = "QUERY>"
	c.Layout = LayoutTypeTopDown
	c.Use256Color = false
	return nil
}

// ReadFilename reads the config from the given file, and
// does the appropriate processing, if any
func (c *Config) ReadFilename(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return errors.Wrapf(err, "failed to open file %s", filename)
	}
	defer f.Close()

	switch ext := filepath.Ext(filename); ext {
	case ".yaml", ".yml":
		err = yaml.NewDecoder(f).Decode(c)
		if err != nil {
			return errors.Wrap(err, "failed to decode YAML")
		}
	default:
		err = json.NewDecoder(f).Decode(c)
		if err != nil {
			return errors.Wrap(err, "failed to decode JSON")
		}
	}

	if !IsValidLayoutType(LayoutType(c.Layout)) {
		return errors.Errorf("invalid layout type: %s", c.Layout)
	}

	if len(c.CustomMatcher) > 0 {
		fmt.Fprintf(os.Stderr, "'CustomMatcher' is deprecated. Use CustomFilter instead\n")

		for n, cfg := range c.CustomMatcher {
			if _, ok := c.CustomFilter[n]; ok {
				return errors.Errorf("failed to create CustomFilter: '%s' already exists. Refusing to overwrite with deprecated CustomMatcher config", n)
			}

			c.CustomFilter[n] = CustomFilterConfig{
				Cmd:             cfg[0],
				Args:            cfg[1:],
				BufferThreshold: filter.DefaultCustomFilterBufferThreshold,
			}
		}
	}

	return nil
}

var (
	stringToFg = map[string]Attribute{
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
	stringToBg = map[string]Attribute{
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
	stringToFgAttr = map[string]Attribute{
		"bold":      AttrBold,
		"underline": AttrUnderline,
		"reverse":   AttrReverse,
	}
	stringToBgAttr = map[string]Attribute{
		"on_bold": AttrBold,
	}
)

// NewStyleSet creates a new StyleSet struct
func NewStyleSet() *StyleSet {
	ss := &StyleSet{}
	ss.Init()
	return ss
}

func (ss *StyleSet) Init() {
	ss.Basic.fg = ColorDefault
	ss.Basic.bg = ColorDefault
	ss.Query.fg = ColorDefault
	ss.Query.bg = ColorDefault
	ss.Matched.fg = ColorCyan
	ss.Matched.bg = ColorDefault
	ss.SavedSelection.fg = ColorBlack | AttrBold
	ss.SavedSelection.bg = ColorCyan
	ss.Selected.fg = ColorDefault | AttrUnderline
	ss.Selected.bg = ColorMagenta
}

// UnmarshalJSON satisfies json.RawMessage.
func (s *Style) UnmarshalJSON(buf []byte) error {
	raw := []string{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return errors.Wrapf(err, "failed to unmarshal Style")
	}
	return stringsToStyle(s, raw)
}

// UnmarshalYAML decodes a YAML array of strings into a Style.
func (s *Style) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw []string
	if err := unmarshal(&raw); err != nil {
		return errors.Wrap(err, "failed to unmarshal Style from YAML")
	}
	return stringsToStyle(s, raw)
}

func stringsToStyle(style *Style, raw []string) error {
	style.fg = ColorDefault
	style.bg = ColorDefault

	for _, s := range raw {
		fg, ok := stringToFg[s]
		if ok {
			style.fg = fg
		} else if strings.HasPrefix(s, "#") && len(s) == 7 {
			if rgb, err := strconv.ParseUint(s[1:], 16, 32); err == nil {
				style.fg = Attribute(rgb) | AttrTrueColor
			}
		} else {
			if fg, err := strconv.ParseUint(s, 10, 8); err == nil {
				style.fg = Attribute(fg + 1)
			}
		}

		bg, ok := stringToBg[s]
		if ok {
			style.bg = bg
		} else if strings.HasPrefix(s, "on_#") && len(s) == 10 {
			if rgb, err := strconv.ParseUint(s[4:], 16, 32); err == nil {
				style.bg = Attribute(rgb) | AttrTrueColor
			}
		} else {
			if strings.HasPrefix(s, "on_") {
				if bg, err := strconv.ParseUint(s[3:], 10, 8); err == nil {
					style.bg = Attribute(bg + 1)
				}
			}
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

	return nil
}

// This is a variable because we want to change its behavior
// when we run tests.
type configLocateFunc func(string) (string, error)

var configFilenames = []string{"config.json", "config.yaml", "config.yml"}

func locateRcfileIn(dir string) (string, error) {
	for _, basename := range configFilenames {
		file := filepath.Join(dir, basename)
		if _, err := os.Stat(file); err == nil {
			return file, nil
		}
	}
	return "", errors.Errorf("config file not found in %s", dir)
}

// LocateRcfile attempts to find the config file in various locations
func LocateRcfile(locater configLocateFunc) (string, error) {
	// http://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html
	//
	// Try in this order:
	//	  $XDG_CONFIG_HOME/peco/config.{json,yaml,yml}
	//    $XDG_CONFIG_DIR/peco/config.{json,yaml,yml} (where XDG_CONFIG_DIR is listed in $XDG_CONFIG_DIRS)
	//	  ~/.peco/config.{json,yaml,yml}

	home, uErr := homedirFunc()

	// Try dir supplied via env var
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		if file, err := locater(filepath.Join(dir, "peco")); err == nil {
			return file, nil
		}
	} else if uErr == nil { // silently ignore failure for homedir()
		// Try "default" XDG location, is user is available
		if file, err := locater(filepath.Join(home, ".config", "peco")); err == nil {
			return file, nil
		}
	}

	// this standard does not take into consideration windows (duh)
	// while the spec says use ":" as the separator, Go provides us
	// with filepath.ListSeparator, so use it
	if dirs := os.Getenv("XDG_CONFIG_DIRS"); dirs != "" {
		for _, dir := range strings.Split(dirs, fmt.Sprintf("%c", filepath.ListSeparator)) {
			if file, err := locater(filepath.Join(dir, "peco")); err == nil {
				return file, nil
			}
		}
	}

	if uErr == nil { // silently ignore failure for homedir()
		if file, err := locater(filepath.Join(home, ".peco")); err == nil {
			return file, nil
		}
	}

	return "", errors.New("config file not found")
}
