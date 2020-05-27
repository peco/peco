package peco

import (
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"context"

	termbox "github.com/nsf/termbox-go"
	"github.com/peco/peco/ui"
	"github.com/stretchr/testify/assert"
)

func TestIssue212_SanityCheck(t *testing.T) {
	state := newPeco()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = state.Run(ctx) }()
	defer cancel()

	<-state.Ready()

	// Check if the default layout type is honored */
	// This the main issue on 212, but while we're at it, we're just
	// going to check that all the default values are as expected
	if !assert.Equal(t, state.config.Layout, "top-down", "Default layout type should be 'top-down', got '%s'", state.config.Layout) {
		return
	}

	if !assert.Equal(t, len(state.config.Keymap), 0, "Default keymap should be empty, but got '%#v'", state.config.Keymap) {
		return
	}

	if !assert.Equal(t, state.config.InitialMatcher, IgnoreCaseMatch, "Default matcher should be IgnoreCaseMatch, but got '%s'", state.config.InitialMatcher) {
		return
	}

	defstyle := ui.NewStyleSet()
	if !assert.Equal(t, state.config.Style, defstyle, "should be default style") {
		return
	}

	if !assert.Equal(t, state.config.Prompt, "QUERY>", "Default prompt should be 'QUERY>', but got '%s'", state.config.Prompt) {
		return
	}

	// Okay, this time create a dummy config file, and read that in
	f, err := ioutil.TempFile("", "peco-test-config")
	if !assert.NoError(t, err, "Failed to create temporary config file: %s", err) {
		return
	}
	fn := f.Name()
	defer os.Remove(fn)

	_, _ = io.WriteString(f, `{
    "Layout": "bottom-up"
}`)
	f.Close()

	state = newPeco()
	go func() { _ = state.Run(ctx) }()

	<-state.Ready()

	if !assert.NoError(t, state.config.ReadFilename(fn), "Failed to read config: %s", err) {
		return
	}
	if !assert.Equal(t, state.config.Layout, "bottom-up", "Default layout type should be 'bottom-up', got '%s'", state.config.Layout) {
		return
	}
}

func TestIssue345(t *testing.T) {
	cfg, err := newConfig(`{
	"Keymap": {
    "C-t": "my.ToggleSelectionInAboveLine"
	},
	"Action": {
		"my.ToggleSelectionInAboveLine": [
			"peco.SelectUp",
			"peco.ToggleSelectionAndSelectNext"
		]
	}
}`)
	if !assert.NoError(t, err, "newConfig should succeed") {
		return
	}
	defer os.Remove(cfg)

	state := newPeco()
	state.skipReadConfig = false
	if !assert.NoError(t, state.config.Init(), "Config.Init should succeed") {
		return
	}

	state.Argv = append(state.Argv, []string{"--rcfile", cfg}...)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = state.Run(ctx) }()
	defer cancel()

	<-state.Ready()

	ev := termbox.Event{
		Type: termbox.EventKey,
		Key:  termbox.KeyCtrlT,
	}
	if !assert.NoError(t, state.Keymap().ExecuteAction(ctx, state, ev), "ExecuteAction should succeed") {
		return
	}

	time.Sleep(time.Second)

}
