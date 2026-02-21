package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// ColorMode specifies how peco handles ANSI color codes in input.
type ColorMode string

const (
	ColorModeAuto ColorMode = "auto"
	ColorModeNone ColorMode = "none"
)

func (c *ColorMode) unmarshal(s string) error {
	switch s {
	case "", "auto":
		*c = ColorModeAuto
	case "none":
		*c = ColorModeNone
	default:
		return fmt.Errorf("invalid Color value %q: must be %q or %q", s, ColorModeAuto, ColorModeNone)
	}
	return nil
}

// UnmarshalText implements encoding.TextUnmarshaler (used by JSON/YAML decoders).
func (c *ColorMode) UnmarshalText(b []byte) error {
	return c.unmarshal(string(b))
}

// UnmarshalFlag implements go-flags Unmarshaler (used by CLI flag parsing).
func (c *ColorMode) UnmarshalFlag(s string) error {
	return c.unmarshal(s)
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
	Color               ColorMode                     `json:"Color" yaml:"Color"`

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

// DefaultPrompt is the default prompt string shown in the query line.
const DefaultPrompt = "QUERY>"

var homedirFunc = util.Homedir

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

	if !IsValidLayoutType(c.Layout) {
		return fmt.Errorf("invalid layout type: %s", c.Layout)
	}

	return nil
}

// Locator locates a config file in a given directory.
type Locator interface {
	Locate(string) (string, error)
}

// LocatorFunc is a function that implements Locator.
type LocatorFunc func(string) (string, error)

// Locate calls the underlying function.
func (f LocatorFunc) Locate(dir string) (string, error) {
	return f(dir)
}

var configFilenames = []string{"config.json", "config.yaml", "config.yml"}

// DefaultConfigLocator searches for a config file with one of the known
// filenames (config.json, config.yaml, config.yml) in the given directory.
var DefaultConfigLocator = LocatorFunc(func(dir string) (string, error) {
	for _, basename := range configFilenames {
		file := filepath.Join(dir, basename)
		if _, err := os.Stat(file); err == nil {
			return file, nil
		}
	}
	return "", fmt.Errorf("config file not found in %s", dir)
})

// LocateRcfile attempts to find the config file in various locations
func LocateRcfile(locater Locator) (string, error) {
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
