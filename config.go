package peco

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/peco/peco/internal/util"
)

// OnCancelBehavior specifies what happens when the user cancels peco.
type OnCancelBehavior string

const (
	OnCancelSuccess OnCancelBehavior = "success"
	OnCancelError   OnCancelBehavior = "error"
)

// UnmarshalText parses a text value into an OnCancelBehavior, accepting
// "success" (or empty) and "error" as valid values.
func (o *OnCancelBehavior) UnmarshalText(b []byte) error {
	switch s := string(b); s {
	case "", "success":
		*o = OnCancelSuccess
	case "error":
		*o = OnCancelError
	default:
		return fmt.Errorf("invalid OnCancel value %q: must be %q or %q", s, OnCancelSuccess, OnCancelError)
	}
	return nil
}

// Config holds all the data that can be configured in the
// external configuration file
type Config struct {
	Action map[string][]string `json:"Action" yaml:"Action"`
	// Keymap used to be directly responsible for dispatching
	// events against user input, but since then this has changed
	// into something that just records the user's config input
	Keymap              map[string]string             `json:"Keymap" yaml:"Keymap"`
	InitialFilter       string                        `json:"InitialFilter" yaml:"InitialFilter"`
	Style               StyleSet                      `json:"Style" yaml:"Style"`
	Prompt              string                        `json:"Prompt" yaml:"Prompt"`
	Layout              string                        `json:"Layout" yaml:"Layout"`
	OnCancel            OnCancelBehavior              `json:"OnCancel" yaml:"OnCancel"`
	CustomFilter        map[string]CustomFilterConfig `json:"CustomFilter" yaml:"CustomFilter"`
	QueryExecutionDelay int                           `json:"QueryExecutionDelay" yaml:"QueryExecutionDelay"`
	StickySelection     bool                          `json:"StickySelection" yaml:"StickySelection"`
	MaxScanBufferSize   int                           `json:"MaxScanBufferSize" yaml:"MaxScanBufferSize"`
	FilterBufSize       int                           `json:"FilterBufSize" yaml:"FilterBufSize"`
	FuzzyLongestSort    bool                          `json:"FuzzyLongestSort" yaml:"FuzzyLongestSort"`
	SuppressStatusMsg   bool                          `json:"SuppressStatusMsg" yaml:"SuppressStatusMsg"`
	ANSI                bool                          `json:"ANSI" yaml:"ANSI"`

	// If this is true, then the prefix for single key jump mode
	// is displayed by default.
	SingleKeyJump SingleKeyJumpConfig `json:"SingleKeyJump" yaml:"SingleKeyJump"`

	// Use this prefix to denote currently selected line
	SelectionPrefix string `json:"SelectionPrefix" yaml:"SelectionPrefix"`

	// Height specifies the display height in lines or percentage (e.g. "10", "50%").
	// When set, peco renders inline without using the alternate screen buffer.
	Height string `json:"Height" yaml:"Height"`
}

// SingleKeyJumpConfig holds configuration for single key jump mode.
type SingleKeyJumpConfig struct {
	ShowPrefix bool `json:"ShowPrefix" yaml:"ShowPrefix"`
}

// CustomFilterConfig is used to specify configuration parameters
// to CustomFilters
type CustomFilterConfig struct {
	// Cmd is the name of the command to invoke
	Cmd string `json:"Cmd" yaml:"Cmd"`

	// TODO: need to check if how we use this is correct
	Args []string `json:"Args" yaml:"Args"`

	// BufferThreshold defines how many lines peco buffers before
	// invoking the external command. If this value is big, we
	// will execute the external command fewer times, but the
	// results will not be generated for longer periods of time.
	// If this value is small, we will execute the external command
	// more often, but you pay the penalty of invoking that command
	// more times.
	BufferThreshold int `json:"BufferThreshold" yaml:"BufferThreshold"`
}

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
	fg Attribute
	bg Attribute
}

var homedirFunc = util.Homedir

// DefaultPrompt is the default prompt string shown in the query line.
const DefaultPrompt = "QUERY>"

// Init initializes the Config with default values
func (c *Config) Init() error {
	c.Keymap = make(map[string]string)
	c.Style.Init()
	c.Prompt = DefaultPrompt
	c.Layout = LayoutTypeTopDown
	return nil
}

// ReadFilename reads the config from the given file, and
// does the appropriate processing, if any
func (c *Config) ReadFilename(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer f.Close()

	switch ext := filepath.Ext(filename); ext {
	case ".yaml", ".yml":
		err = yaml.NewDecoder(f).Decode(c)
		if err != nil {
			return fmt.Errorf("failed to decode YAML: %w", err)
		}
	default:
		err = json.NewDecoder(f).Decode(c)
		if err != nil {
			return fmt.Errorf("failed to decode JSON: %w", err)
		}
	}

	if !IsValidLayoutType(LayoutType(c.Layout)) {
		return fmt.Errorf("invalid layout type: %s", c.Layout)
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

// Init initializes the StyleSet with default foreground and background colors
// for each UI element (basic, query, matched, selected, prompt, context, etc.).
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
	ss.Prompt.fg = ColorDefault
	ss.Prompt.bg = ColorDefault
	ss.Context.fg = ColorDefault | AttrBold
	ss.Context.bg = ColorDefault
}

// UnmarshalJSON satisfies json.RawMessage.
func (s *Style) UnmarshalJSON(buf []byte) error {
	raw := []string{}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return fmt.Errorf("failed to unmarshal Style: %w", err)
	}
	return stringsToStyle(s, raw)
}

// UnmarshalYAML decodes a YAML array of strings into a Style.
func (s *Style) UnmarshalYAML(unmarshal func(any) error) error {
	var raw []string
	if err := unmarshal(&raw); err != nil {
		return fmt.Errorf("failed to unmarshal Style from YAML: %w", err)
	}
	return stringsToStyle(s, raw)
}

// stringsToStyle parses an array of color and attribute strings (e.g. "red",
// "on_blue", "bold", "#ff00ff") into a Style's foreground and background Attributes.
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

// ConfigLocator locates a config file in a given directory.
type ConfigLocator interface {
	Locate(string) (string, error)
}

// ConfigLocatorFunc is a function that implements ConfigLocator.
type ConfigLocatorFunc func(string) (string, error)

// Locate calls the underlying function.
func (f ConfigLocatorFunc) Locate(dir string) (string, error) {
	return f(dir)
}

var configFilenames = []string{"config.json", "config.yaml", "config.yml"}

// defaultConfigLocator searches for a config file with one of the known
// filenames (config.json, config.yaml, config.yml) in the given directory.
var defaultConfigLocator = ConfigLocatorFunc(func(dir string) (string, error) {
	for _, basename := range configFilenames {
		file := filepath.Join(dir, basename)
		if _, err := os.Stat(file); err == nil {
			return file, nil
		}
	}
	return "", fmt.Errorf("config file not found in %s", dir)
})

// LocateRcfile attempts to find the config file in various locations
func LocateRcfile(locater ConfigLocator) (string, error) {
	// http://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html
	//
	// Try in this order:
	//	  $XDG_CONFIG_HOME/peco/config.{json,yaml,yml}
	//    $XDG_CONFIG_DIR/peco/config.{json,yaml,yml} (where XDG_CONFIG_DIR is listed in $XDG_CONFIG_DIRS)
	//	  ~/.peco/config.{json,yaml,yml}

	home, uErr := homedirFunc()

	// Try dir supplied via env var
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		if file, err := locater.Locate(filepath.Join(dir, "peco")); err == nil {
			return file, nil
		}
	} else if uErr == nil { // silently ignore failure for homedir()
		// Try "default" XDG location, is user is available
		if file, err := locater.Locate(filepath.Join(home, ".config", "peco")); err == nil {
			return file, nil
		}
	}

	// this standard does not take into consideration windows (duh)
	// while the spec says use ":" as the separator, Go provides us
	// with filepath.ListSeparator, so use it
	if dirs := os.Getenv("XDG_CONFIG_DIRS"); dirs != "" {
		for dir := range strings.SplitSeq(dirs, fmt.Sprintf("%c", filepath.ListSeparator)) {
			if file, err := locater.Locate(filepath.Join(dir, "peco")); err == nil {
				return file, nil
			}
		}
	}

	if uErr == nil { // silently ignore failure for homedir()
		if file, err := locater.Locate(filepath.Join(home, ".peco")); err == nil {
			return file, nil
		}
	}

	return "", errors.New("config file not found")
}
