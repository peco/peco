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
		Layout:         DefaultLayoutType,
		Prompt:         "[peco]",
		Style: StyleSet{
			Matched: Style{
				fg: termbox.ColorCyan | termbox.AttrBold,
				bg: termbox.ColorRed,
			},
			Query: Style{
				fg: termbox.ColorYellow | termbox.AttrBold,
				bg: termbox.ColorDefault,
			},
			Selected: Style{
				fg: termbox.ColorBlack | termbox.AttrUnderline,
				bg: termbox.ColorCyan,
			},
			SavedSelection: Style{
				fg: termbox.ColorBlack | termbox.AttrBold,
				bg: termbox.ColorCyan,
			},
		},
	}

	if !assert.Equal(t, expected, cfg, "configuration matches expected") {
		return
	}
}

type stringsToStyleTest struct {
	strings []string
	style   *Style
}

func TestStringsToStyle(t *testing.T) {
	tests := []stringsToStyleTest{
		stringsToStyleTest{
			strings: []string{"on_default", "default"},
			style:   &Style{fg: termbox.ColorDefault, bg: termbox.ColorDefault},
		},
		stringsToStyleTest{
			strings: []string{"bold", "on_blue", "yellow"},
			style:   &Style{fg: termbox.ColorYellow | termbox.AttrBold, bg: termbox.ColorBlue},
		},
		stringsToStyleTest{
			strings: []string{"underline", "on_cyan", "black"},
			style:   &Style{fg: termbox.ColorBlack | termbox.AttrUnderline, bg: termbox.ColorCyan},
		},
		stringsToStyleTest{
			strings: []string{"reverse", "on_red", "white"},
			style:   &Style{fg: termbox.ColorWhite | termbox.AttrReverse, bg: termbox.ColorRed},
		},
		stringsToStyleTest{
			strings: []string{"on_bold", "on_magenta", "green"},
			style:   &Style{fg: termbox.ColorGreen, bg: termbox.ColorMagenta | termbox.AttrBold},
		},
	}

	t.Logf("Checking strings -> color mapping...")
	var a Style
	for _, test := range tests {
		t.Logf("    checking %s...", test.strings)
		if !assert.NoError(t, stringsToStyle(&a, test.strings), "stringsToStyle should succeed") {
			return
		}

		if !assert.Equal(t, test.style, &a, "Expected '%s' to be '%#v', but got '%#v'", test.strings, test.style, a) {
			return
		}
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

	LocateRcfile(locater)
	expected[0] = filepath.Join(dir, ".config", "peco")
	os.Setenv("XDG_CONFIG_HOME", "")
	i = 0
	LocateRcfile(locater)

}
