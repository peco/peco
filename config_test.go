package peco

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/ui"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestReadRC(t *testing.T) {
	txt := `
{
	"Keymap": {
		"C-j": "peco.Finish",
		"C-x,C-c": "peco.Finish"
	},
	"Style": {
		"Basic": ["on_default", "default"],
		"Selected": ["underline", "on_cyan", "black"],
		"Query": ["yellow", "bold"],
		"Matched": ["cyan", "bold", "on_red"]
	},
	"Prompt": "[peco]"
}
`
	var cfg Config
	if !assert.NoError(t, cfg.Init(), "Config.Init should succeed") {
		return
	}

	if !assert.NoError(t, json.Unmarshal([]byte(txt), &cfg), "Unmarshalling config should succeed") {
		return
	}

	expected := Config{
		Keymap: map[string]string{
			"C-j":     "peco.Finish",
			"C-x,C-c": "peco.Finish",
		},
		InitialMatcher: IgnoreCaseMatch,
		Layout:         ui.DefaultLayoutType,
		Prompt:         "[peco]",
		Style: &ui.StyleSet{
			Basic:          ui.NewStyle(termbox.ColorDefault, termbox.ColorDefault),
			Matched:        ui.NewStyle(termbox.ColorCyan|termbox.AttrBold, termbox.ColorRed),
			Query:          ui.NewStyle(termbox.ColorYellow|termbox.AttrBold, termbox.ColorDefault),
			Selected:       ui.NewStyle(termbox.ColorBlack|termbox.AttrUnderline, termbox.ColorCyan),
			SavedSelection: ui.NewStyle(termbox.ColorBlack|termbox.AttrBold, termbox.ColorCyan),
		},
	}

	if !assert.Equal(t, expected, cfg, "configuration matches expected") {
		return
	}
}

func TestLocateRcfile(t *testing.T) {
	dir, err := ioutil.TempDir("", "peco-")
	if !assert.NoError(t, err, "Failed to create temporary directory: %s", err) {
		return
	}

	homedirFunc = func() (string, error) {
		return dir, nil
	}

	expected := []string{
		filepath.Join(dir, "peco"),
		filepath.Join(dir, "1", "peco"),
		filepath.Join(dir, "2", "peco"),
		filepath.Join(dir, "3", "peco"),
		filepath.Join(dir, ".peco"),
	}

	i := 0
	locater := func(dir string) (string, error) {
		t.Logf("looking for file in %s", dir)
		if !assert.True(t, i <= len(expected)-1, "Got %d directories, only have %d", i+1, len(expected)) {
			return "", errors.New("error: Not found")
		}

		if !assert.Equal(t, expected[i], dir, "Expected %s, got %s", expected[i], dir) {
			return "", errors.New("error: Not found")
		}
		i++
		return "", errors.New("error: Not found")
	}

	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Setenv("XDG_CONFIG_DIRS", strings.Join(
		[]string{
			filepath.Join(dir, "1"),
			filepath.Join(dir, "2"),
			filepath.Join(dir, "3"),
		},
		fmt.Sprintf("%c", filepath.ListSeparator),
	))

	_, _ = LocateRcfile(locater)
	expected[0] = filepath.Join(dir, ".config", "peco")
	os.Setenv("XDG_CONFIG_HOME", "")
	i = 0
	_, _ = LocateRcfile(locater)

}
