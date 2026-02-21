package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/require"
)

var expectedConfig = Config{
	Keymap: map[string]string{
		"C-j":     "peco.Finish",
		"C-x,C-c": "peco.Finish",
	},
	Layout: DefaultLayoutType,
	Prompt: "[peco]",
	Style: StyleSet{
		Matched: Style{
			Fg: ColorCyan | AttrBold,
			Bg: ColorRed,
		},
		Query: Style{
			Fg: ColorYellow | AttrBold,
			Bg: ColorDefault,
		},
		Selected: Style{
			Fg: ColorBlack | AttrUnderline,
			Bg: ColorCyan,
		},
		SavedSelection: Style{
			Fg: ColorBlack | AttrBold,
			Bg: ColorCyan,
		},
		Prompt: Style{
			Fg: ColorGreen | AttrBold,
			Bg: ColorDefault,
		},
		Context: Style{
			Fg: ColorDefault | AttrBold,
			Bg: ColorDefault,
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
		{
			strings: []string{"on_default", "default"},
			style:   &Style{Fg: ColorDefault, Bg: ColorDefault},
		},
		{
			strings: []string{"bold", "on_blue", "yellow"},
			style:   &Style{Fg: ColorYellow | AttrBold, Bg: ColorBlue},
		},
		{
			strings: []string{"underline", "on_cyan", "black"},
			style:   &Style{Fg: ColorBlack | AttrUnderline, Bg: ColorCyan},
		},
		{
			strings: []string{"reverse", "on_red", "white"},
			style:   &Style{Fg: ColorWhite | AttrReverse, Bg: ColorRed},
		},
		{
			strings: []string{"on_bold", "on_magenta", "green"},
			style:   &Style{Fg: ColorGreen, Bg: ColorMagenta | AttrBold},
		},
		{
			strings: []string{"underline", "on_240", "214"},
			style:   &Style{Fg: Attribute(214+1) | AttrUnderline, Bg: Attribute(240 + 1)},
		},
		{
			strings: []string{"#ff8800", "on_#0088ff"},
			style:   &Style{Fg: Attribute(0xff8800) | AttrTrueColor, Bg: Attribute(0x0088ff) | AttrTrueColor},
		},
		{
			strings: []string{"bold", "#00ff00", "on_#000000"},
			style:   &Style{Fg: Attribute(0x00ff00) | AttrTrueColor | AttrBold, Bg: Attribute(0x000000) | AttrTrueColor},
		},
		{
			strings: []string{"#000000"},
			style:   &Style{Fg: Attribute(0x000000) | AttrTrueColor, Bg: ColorDefault},
		},
	}

	t.Logf("Checking strings -> color mapping...")
	var a Style
	for _, test := range tests {
		t.Logf("    checking %s...", test.strings)
		require.NoError(t, StringsToStyle(&a, test.strings), "StringsToStyle should succeed")
		require.Equal(t, test.style, &a, "Expected '%s' to be '%#v', but got '%#v'", test.strings, test.style, a)
	}
}

func TestLocateRcfile(t *testing.T) {
	dir := t.TempDir()

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
	locater := LocatorFunc(func(dir string) (string, error) {
		t.Logf("looking for file in %s", dir)
		require.True(t, i <= len(expected)-1, "Got %d directories, only have %d", i+1, len(expected))
		require.Equal(t, expected[i], dir, "Expected %s, got %s", expected[i], dir)
		i++
		return "", errors.New("error: Not found")
	})

	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CONFIG_DIRS", strings.Join(
		[]string{
			filepath.Join(dir, "1"),
			filepath.Join(dir, "2"),
			filepath.Join(dir, "3"),
		},
		fmt.Sprintf("%c", filepath.ListSeparator),
	))

	LocateRcfile(locater)
	expected[0] = filepath.Join(dir, ".config", "peco")
	t.Setenv("XDG_CONFIG_HOME", "")
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
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CONFIG_DIRS", "")

	file, err := LocateRcfile(DefaultConfigLocator)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(pecoDir, "config.yaml"), file)
}

func TestOnCancelBehavior(t *testing.T) {
	t.Run("valid values via JSON", func(t *testing.T) {
		for _, tc := range []struct {
			input    string
			expected OnCancelBehavior
		}{
			{`{"OnCancel":"success"}`, OnCancelSuccess},
			{`{"OnCancel":"error"}`, OnCancelError},
			{`{}`, ""}, // absent key stays at zero value; default applied later in ApplyConfig
		} {
			var cfg Config
			require.NoError(t, cfg.Init())
			require.NoError(t, json.Unmarshal([]byte(tc.input), &cfg))
			require.Equal(t, tc.expected, cfg.OnCancel)
		}
	})

	t.Run("valid values via YAML", func(t *testing.T) {
		for _, tc := range []struct {
			input    string
			expected OnCancelBehavior
		}{
			{"OnCancel: success", OnCancelSuccess},
			{"OnCancel: error", OnCancelError},
		} {
			var cfg Config
			require.NoError(t, cfg.Init())
			require.NoError(t, yaml.Unmarshal([]byte(tc.input), &cfg))
			require.Equal(t, tc.expected, cfg.OnCancel)
		}
	})

	t.Run("invalid value via JSON", func(t *testing.T) {
		var cfg Config
		require.NoError(t, cfg.Init())
		err := json.Unmarshal([]byte(`{"OnCancel":"bogus"}`), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "bogus")
	})

	t.Run("invalid value via YAML", func(t *testing.T) {
		var cfg Config
		require.NoError(t, cfg.Init())
		err := yaml.Unmarshal([]byte("OnCancel: bogus"), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "bogus")
	})
}

func TestColorMode(t *testing.T) {
	t.Run("valid values via JSON", func(t *testing.T) {
		for _, tc := range []struct {
			input    string
			expected ColorMode
		}{
			{`{"Color":"auto"}`, ColorModeAuto},
			{`{"Color":"none"}`, ColorModeNone},
			{`{}`, ""}, // absent key stays at zero value; default applied later in ApplyConfig
		} {
			var cfg Config
			require.NoError(t, cfg.Init())
			require.NoError(t, json.Unmarshal([]byte(tc.input), &cfg))
			require.Equal(t, tc.expected, cfg.Color)
		}
	})

	t.Run("valid values via YAML", func(t *testing.T) {
		for _, tc := range []struct {
			input    string
			expected ColorMode
		}{
			{"Color: auto", ColorModeAuto},
			{"Color: none", ColorModeNone},
		} {
			var cfg Config
			require.NoError(t, cfg.Init())
			require.NoError(t, yaml.Unmarshal([]byte(tc.input), &cfg))
			require.Equal(t, tc.expected, cfg.Color)
		}
	})

	t.Run("invalid value via JSON", func(t *testing.T) {
		var cfg Config
		require.NoError(t, cfg.Init())
		err := json.Unmarshal([]byte(`{"Color":"bogus"}`), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "bogus")
	})

	t.Run("invalid value via YAML", func(t *testing.T) {
		var cfg Config
		require.NoError(t, cfg.Init())
		err := yaml.Unmarshal([]byte("Color: bogus"), &cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "bogus")
	})

	t.Run("UnmarshalFlag valid values", func(t *testing.T) {
		var c ColorMode
		require.NoError(t, c.UnmarshalFlag("auto"))
		require.Equal(t, ColorModeAuto, c)

		require.NoError(t, c.UnmarshalFlag("none"))
		require.Equal(t, ColorModeNone, c)

		require.NoError(t, c.UnmarshalFlag(""))
		require.Equal(t, ColorModeAuto, c)
	})

	t.Run("UnmarshalFlag invalid value", func(t *testing.T) {
		var c ColorMode
		err := c.UnmarshalFlag("bogus")
		require.Error(t, err)
		require.Contains(t, err.Error(), "bogus")
	})
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
