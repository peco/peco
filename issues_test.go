package peco

import (
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

func TestIssue212(t *testing.T) {
	ctx := NewCtx(nil)

	// Check if the default layout type is honored */
	// This the main issue on 212, but while we're at it, we're just
	// going to check that all the default values are as expected
	if ctx.config.Layout != "top-down" {
		t.Errorf("Default layout type should be 'top-down', got '%s'", ctx.config.Layout)
	}

	if len(ctx.config.Keymap) != 0 {
		t.Errorf("Default keymap should be empty, but got '%#v'", ctx.config.Keymap)
	}

	if ctx.config.InitialMatcher != IgnoreCaseMatch {
		t.Errorf("Default matcher should IgnoreCaseMatch, but got '%s'", ctx.config.InitialMatcher)
	}

	if !reflect.DeepEqual(ctx.config.Style, NewStyleSet()) {
		t.Errorf("Default style should was not the same as NewStyleSet()")
	}

	if ctx.config.Prompt != "QUERY>" {
		t.Errorf("Default prompt should be 'QUERY>', but got '%s'", ctx.config.Prompt)
	}

	// Okay, this time create a dummy config file, and read that in
	f, err := ioutil.TempFile("", "peco-test-config")
	if err != nil {
		t.Errorf("Failed to create temporary config file: %s", err)
		return
	}
	fn := f.Name()
	defer os.Remove(fn)

	io.WriteString(f, `{
    "Layout": "bottom-up"
}`)
	f.Close()

	ctx = NewCtx(nil)
	if err := ctx.ReadConfig(fn); err != nil {
		t.Errorf("Failed to read config: %s", err)
		return
	}
	if ctx.config.Layout != "bottom-up" {
		t.Errorf("Default layout type should be 'bottom-up', got '%s'", ctx.config.Layout)
	}
}