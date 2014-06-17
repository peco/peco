package peco

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/nsf/termbox-go"
)

var currentUser = user.Current
type Config struct {
	Keymap  Keymap   `json:"Keymap"`
	Matcher string   `json:"Matcher"`
	Style   StyleSet `json:"Style"`
}

func NewConfig() *Config {
	return &Config{
		Keymap:  NewKeymap(),
		Matcher: IgnoreCaseMatch,
		Style:   NewStyleSet(),
	}
}

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
	stringToAttr = map[string]termbox.Attribute{
		"bold":      termbox.AttrBold,
		"underline": termbox.AttrUnderline,
		"blink":     termbox.AttrReverse,
	}
)

type StyleSet struct {
	Basic          Style `json:"Basic"`
	SavedSelection Style `json:"SavedSelection"`
	Selected       Style `json:"Selected"`
	Query          Style `json:"Query"`
}

func NewStyleSet() StyleSet {
	return StyleSet{
		Basic:          Style{fg: termbox.ColorDefault, bg: termbox.ColorDefault},
		SavedSelection: Style{fg: termbox.ColorBlack | termbox.AttrBold, bg: termbox.ColorCyan},
		Selected:       Style{fg: termbox.ColorDefault | termbox.AttrUnderline, bg: termbox.ColorMagenta},
		Query:          Style{fg: termbox.ColorCyan, bg: termbox.ColorDefault},
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

var _locateRcfileIn = locateRcfileIn
func locateRcfileIn(dir string) (string, error) {
	const basename = "config.json"
	file := filepath.Join(dir, basename)
fmt.Fprintf(os.Stderr, "Looking for %s\n", file)
	if _, err := os.Stat(file); err != nil {
		return "", err
	}
	return file, nil
}

func LocateRcfile() (string, error) {
	// http://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html
	//
	// Try in this order:
	//	  $XDG_CONFIG_HOME/peco/config.json
	//    $XDG_CONFIG_DIR/peco/config.json (where XDG_CONFIG_DIR is listed in $XDG_CONFIG_DIRS)
	//	  ~/.peco/config.json

	user, uErr := currentUser()

	// Try dir supplied via env var
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		file, err := _locateRcfileIn(filepath.Join(dir, "peco"))
		if err == nil {
			return file, nil
		}
	} else if uErr == nil { // silently ignore failure for user.Current()
		// Try "default" XDG location, is user is available
		file, err := _locateRcfileIn(filepath.Join(user.HomeDir, ".config", "peco"))
		if err == nil {
			return file, nil
		}
	}

	// this standard does not take into consideration windows (duh)
	// while the spec says use ":" as the separator, Go provides us
	// with filepath.ListSeparator, so use it
	if dirs := os.Getenv("XDG_CONFIG_DIRS"); dirs != "" {
		for _, dir := range strings.Split(dirs, fmt.Sprintf("%c", filepath.ListSeparator)) {
			file, err := _locateRcfileIn(filepath.Join(dir, "peco"))
			if err == nil {
				return file, nil
			}
		}
	}

	if uErr == nil { // silently ignore failure for user.Current()
		file, err := _locateRcfileIn(filepath.Join(user.HomeDir, ".peco"))
		if err == nil {
			return file, nil
		}
	}

	return "", fmt.Errorf("Config file not found")
}
