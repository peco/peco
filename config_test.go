package peco

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

var expectedConfig = Config{
	Keymap: map[string]string{
		"C-j":      "peco.Finish",
		"C-x,C-c": "peco.Finish",
	},
	InitialMatcher: IgnoreCaseMatch,
	Layout:         DefaultLayoutType,
	Prompt:         "[peco]",
	Style: StyleSet{
		Matched: Style{
			fg: ColorCyan | AttrBold,
			bg: ColorRed,
		},
		Query: Style{
			fg: ColorYellow | AttrBold,
			bg: ColorDefault,
		},
		Selected: Style{
			fg: ColorBlack | AttrUnderline,
			bg: ColorCyan,
		},
		SavedSelection: Style{
			fg: ColorBlack | AttrBold,
			bg: ColorCyan,
		},
		Prompt: Style{
			fg: ColorGreen | AttrBold,
			bg: ColorDefault,
		},
	},
}

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
		"Matched": ["cyan", "bold", "on_red"],
		"Prompt": ["green", "bold"]
	},
	"Prompt": "[peco]"
}
`
	var cfg Config
	require.NoError(t, cfg.Init(), "Config.Init should succeed")
	require.NoError(t, json.Unmarshal([]byte(txt), &cfg), "Unmarshalling config should succeed")
	require.Equal(t, expectedConfig, cfg, "configuration matches expected")
}

func TestReadRCYAML(t *testing.T) {
	txt := `
Keymap:
  C-j: peco.Finish
  "C-x,C-c": peco.Finish
Style:
  Basic:
    - on_default
    - default
  Selected:
    - underline
    - on_cyan
    - black
  Query:
    - yellow
    - bold
  Matched:
    - cyan
    - bold
    - on_red
  Prompt:
    - green
    - bold
Prompt: "[peco]"
`
	var cfg Config
	require.NoError(t, cfg.Init(), "Config.Init should succeed")
	require.NoError(t, yaml.Unmarshal([]byte(txt), &cfg), "Unmarshalling YAML config should succeed")
	require.Equal(t, expectedConfig, cfg, "YAML configuration matches expected")
}

type stringsToStyleTest struct {
	strings []string
	style   *Style
}

func TestStringsToStyle(t *testing.T) {
	tests := []stringsToStyleTest{
		stringsToStyleTest{
			strings: []string{"on_default", "default"},
			style:   &Style{fg: ColorDefault, bg: ColorDefault},
		},
		stringsToStyleTest{
			strings: []string{"bold", "on_blue", "yellow"},
			style:   &Style{fg: ColorYellow | AttrBold, bg: ColorBlue},
		},
		stringsToStyleTest{
			strings: []string{"underline", "on_cyan", "black"},
			style:   &Style{fg: ColorBlack | AttrUnderline, bg: ColorCyan},
		},
		stringsToStyleTest{
			strings: []string{"reverse", "on_red", "white"},
			style:   &Style{fg: ColorWhite | AttrReverse, bg: ColorRed},
		},
		stringsToStyleTest{
			strings: []string{"on_bold", "on_magenta", "green"},
			style:   &Style{fg: ColorGreen, bg: ColorMagenta | AttrBold},
		},
		stringsToStyleTest{
			strings: []string{"underline", "on_240", "214"},
			style:   &Style{fg: Attribute(214+1) | AttrUnderline, bg: Attribute(240 + 1)},
		},
		stringsToStyleTest{
			strings: []string{"#ff8800", "on_#0088ff"},
			style:   &Style{fg: Attribute(0xff8800) | AttrTrueColor, bg: Attribute(0x0088ff) | AttrTrueColor},
		},
		stringsToStyleTest{
			strings: []string{"bold", "#00ff00", "on_#000000"},
			style:   &Style{fg: Attribute(0x00ff00) | AttrTrueColor | AttrBold, bg: Attribute(0x000000) | AttrTrueColor},
		},
		stringsToStyleTest{
			strings: []string{"#000000"},
			style:   &Style{fg: Attribute(0x000000) | AttrTrueColor, bg: ColorDefault},
		},
	}

	t.Logf("Checking strings -> color mapping...")
	var a Style
	for _, test := range tests {
		t.Logf("    checking %s...", test.strings)
		require.NoError(t, stringsToStyle(&a, test.strings), "stringsToStyle should succeed")
		require.Equal(t, test.style, &a, "Expected '%s' to be '%#v', but got '%#v'", test.strings, test.style, a)
	}
}

func TestLocateRcfile(t *testing.T) {
	dir, err := os.MkdirTemp("", "peco-")
	require.NoError(t, err, "Failed to create temporary directory")

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
		require.True(t, i <= len(expected)-1, "Got %d directories, only have %d", i+1, len(expected))
		require.Equal(t, expected[i], dir, "Expected %s, got %s", expected[i], dir)
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

func TestLocateRcfileYAML(t *testing.T) {
	dir := t.TempDir()

	// Create config.yaml (but not config.json) in the dir
	pecoDir := filepath.Join(dir, ".peco")
	require.NoError(t, os.MkdirAll(pecoDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pecoDir, "config.yaml"), []byte("{}"), 0o644))

	homedirFunc = func() (string, error) {
		return dir, nil
	}

	// Clear XDG vars so it falls through to ~/.peco/
	os.Setenv("XDG_CONFIG_HOME", "")
	os.Setenv("XDG_CONFIG_DIRS", "")

	file, err := LocateRcfile(locateRcfileIn)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(pecoDir, "config.yaml"), file)
}

func TestReadFilenameYAML(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte(`
Keymap:
  C-j: peco.Finish
  "C-x,C-c": peco.Finish
Style:
  Basic:
    - on_default
    - default
  Selected:
    - underline
    - on_cyan
    - black
  Query:
    - yellow
    - bold
  Matched:
    - cyan
    - bold
    - on_red
  Prompt:
    - green
    - bold
Prompt: "[peco]"
`), 0o644))

	var cfg Config
	require.NoError(t, cfg.Init())
	require.NoError(t, cfg.ReadFilename(yamlFile))
	require.Equal(t, expectedConfig, cfg)
}
