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
	cfg := NewConfig()
	if err := json.Unmarshal([]byte(txt), cfg); err != nil {
		t.Fatalf("Error unmarshaling json: %s", err)
	}
	t.Logf("%#q", cfg)
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
	for _, test := range tests {
		t.Logf("    checking %s...", test.strings)
		if a := stringsToStyle(test.strings); *a != *test.style {
			t.Errorf("Expected '%s' to be '%#v', but got '%#v'", test.strings, test.style, a)
		}
	}
}

func TestLocateRcfile(t *testing.T) {
	dir, err := ioutil.TempDir("", "peco-")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %s", err)
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
	locateRcfileInFunc = func(dir string) (string, error) {
		t.Logf("looking for file in %s", dir)
		if i > len(expected)-1 {
			t.Fatalf("Got %d directories, only have %d", i+1, len(expected))
		}

		if expected[i] != dir {
			t.Errorf("Expected %s, got %s", expected[i], dir)
		}
		i++
		return "", fmt.Errorf("error: Not found")
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

	LocateRcfile()
	expected[0] = filepath.Join(dir, ".config", "peco")
	os.Setenv("XDG_CONFIG_HOME", "")
	i = 0
	LocateRcfile()

}
