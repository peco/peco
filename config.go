package peco

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peco/peco/filter"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/ui"
	"github.com/pkg/errors"
)

var homedirFunc = util.Homedir

// NewConfig creates a new Config
func (c *Config) Init() error {
	c.Keymap = make(map[string]string)
	c.InitialMatcher = IgnoreCaseMatch
	c.Style = ui.NewStyleSet()
	c.Prompt = "QUERY>"
	c.Layout = ui.LayoutTypeTopDown
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

	err = json.NewDecoder(f).Decode(c)
	if err != nil {
		return errors.Wrap(err, "failed to decode JSON")
	}

	if !ui.IsValidLayoutType(ui.LayoutType(c.Layout)) {
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

// This is a variable because we want to change its behavior
// when we run tests.
type configLocateFunc func(string) (string, error)

func locateRcfileIn(dir string) (string, error) {
	const basename = "config.json"
	file := filepath.Join(dir, basename)
	if _, err := os.Stat(file); err != nil {
		return "", errors.Wrapf(err, "failed to stat file %s", file)
	}
	return file, nil
}

// LocateRcfile attempts to find the config file in various locations
func LocateRcfile(locater configLocateFunc) (string, error) {
	// http://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html
	//
	// Try in this order:
	//	  $XDG_CONFIG_HOME/peco/config.json
	//    $XDG_CONFIG_DIR/peco/config.json (where XDG_CONFIG_DIR is listed in $XDG_CONFIG_DIRS)
	//	  ~/.peco/config.json

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
