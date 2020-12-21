package peco_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lestrrat-go/envload"
	"github.com/peco/peco"
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
	var cfg peco.Config
	if !assert.NoError(t, cfg.Init(), "Config.Init should succeed") {
		return
	}

	if !assert.NoError(t, json.Unmarshal([]byte(txt), &cfg), "Unmarshalling config should succeed") {
		return
	}

	expected := peco.Config{
		Keymap: map[string]string{
			"C-j":     "peco.Finish",
			"C-x,C-c": "peco.Finish",
		},
		InitialMatcher: peco.IgnoreCaseMatch,
		Layout:         peco.DefaultLayoutType,
		Prompt:         "[peco]",
		Style: &peco.StyleSet{
			Basic: peco.NewStyle(),
			Matched: peco.NewStyle().
				Foreground(peco.ColorCyan).
				Background(peco.ColorRed).
				Bold(true),
			Query: peco.NewStyle().
				Foreground(peco.ColorYellow).
				Background(peco.ColorDefault).
				Bold(true),
			Selected: peco.NewStyle().
				Foreground(peco.ColorBlack).
				Background(peco.ColorCyan).
				Underline(true),
			SavedSelection: peco.NewStyle().
				Foreground(peco.ColorBlack).
				Background(peco.ColorCyan).
				Bold(true),
		},
	}

	if !assert.Equal(t, expected, cfg, "configuration matches expected") {
		return
	}
}

type stringsToStyleTest struct {
	strings []string
	style   *peco.Style
}

func TestStringsToStyle(t *testing.T) {
	tests := []stringsToStyleTest{
		stringsToStyleTest{
			strings: []string{"on_default", "default"},
			style: peco.NewStyle().
				Foreground(peco.ColorDefault).
				Background(peco.ColorDefault),
		},
		stringsToStyleTest{
			strings: []string{"bold", "on_blue", "yellow"},
			style: peco.NewStyle().
				Foreground(peco.ColorYellow).
				Background(peco.ColorBlue).
				Bold(true),
		},
		stringsToStyleTest{
			strings: []string{"underline", "on_cyan", "black"},
			style: peco.NewStyle().
				Foreground(peco.ColorBlack).
				Background(peco.ColorCyan).
				Underline(true),
		},
		stringsToStyleTest{
			strings: []string{"reverse", "on_red", "white"},
			style: peco.NewStyle().
				Foreground(peco.ColorWhite).
				Background(peco.ColorRed).
				Reverse(true),
		},
		stringsToStyleTest{
			strings: []string{"on_bold", "on_magenta", "green"},
			style: peco.NewStyle().
				Foreground(peco.ColorGreen).
				Background(peco.ColorMagenta).
				Bold(true),
		},
		stringsToStyleTest{
			strings: []string{"underline", "on_240", "214"},
			style: peco.NewStyle().
				Foreground(214 + 1).
				Background(240 + 1).
				Underline(true),
		},
	}

	var a peco.Style
	for _, tc := range tests {
		tc := tc
		t.Run(strings.Join(tc.strings, ","), func(t *testing.T) {
			if !assert.NoError(t, a.FromStrings(tc.strings...), "stringsToStyle should succeed") {
				return
			}

			if !assert.Equal(t, tc.style, &a, "Expected '%s' to be '%#v', but got '%#v'", tc.strings, tc.style, a) {
				return
			}
		})
	}
}

func TestLocateRcfile(t *testing.T) {
	dir, err := ioutil.TempDir("", "peco-")
	if !assert.NoError(t, err, "Failed to create temporary directory: %s", err) {
		return
	}

	home, err := os.UserHomeDir()
	if !assert.NoError(t, err, `could not find user home directory`) {
		return
	}

	el := envload.New()
	defer el.Restore()

	expected := []string{
		filepath.Join(dir, "peco"),
		filepath.Join(dir, "1", "peco"),
		filepath.Join(dir, "2", "peco"),
		filepath.Join(dir, "3", "peco"),
		filepath.Join(home, ".peco"),
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

	peco.LocateRcfile(locater)
	expected[0] = filepath.Join(home, ".config", "peco")
	os.Setenv("XDG_CONFIG_HOME", "")
	i = 0
	peco.LocateRcfile(locater)

}
