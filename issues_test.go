package peco

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"context"

	"github.com/peco/peco/filter"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue212_SanityCheck(t *testing.T) {
	state := newPeco()
	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
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

	defstyle := StyleSet{}
	defstyle.Init()
	if !assert.Equal(t, state.config.Style, defstyle, "should be default style") {
		return
	}

	if !assert.Equal(t, state.config.Prompt, "QUERY>", "Default prompt should be 'QUERY>', but got '%s'", state.config.Prompt) {
		return
	}

	// Okay, this time create a dummy config file, and read that in
	f, err := os.CreateTemp("", "peco-test-config")
	if !assert.NoError(t, err, "Failed to create temporary config file: %s", err) {
		return
	}
	fn := f.Name()
	defer os.Remove(fn)

	io.WriteString(f, `{
    "Layout": "bottom-up"
}`)
	f.Close()

	state = newPeco()
	go state.Run(ctx)

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
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

	ev := Event{
		Type: EventKey,
		Key:  keyseq.KeyCtrlT,
	}
	if !assert.NoError(t, state.Keymap().ExecuteAction(ctx, state, ev), "ExecuteAction should succeed") {
		return
	}

	time.Sleep(time.Second)

}

// TestIssue557_FilterBufSize verifies that configuring FilterBufSize allows
// FuzzyLongestSort to correctly sort results across what would otherwise be
// separate chunks. With the default buf size of 1000, lines are batched into
// 1000-line chunks and sorting only happens within each chunk. An exact match
// in chunk 2 would always appear after all chunk 1 results.
func TestIssue557_FilterBufSize(t *testing.T) {
	const totalLines = 1100
	const query = "exact"

	// Build lines: 1 partial match, then 1098 non-matching lines, then 1 exact match.
	// The exact match ("exact") is at index 1099, which with the default buf size of
	// 1000 would land in the second chunk.
	var lines []line.Line
	lines = append(lines, line.NewRaw(0, "e_x_a_c_t filler text", false)) // fuzzy match
	for i := 1; i < totalLines-1; i++ {
		lines = append(lines, line.NewRaw(uint64(i), fmt.Sprintf("no match line %d", i), false))
	}
	lines = append(lines, line.NewRaw(uint64(totalLines-1), "exact", false)) // exact match, best result

	f := filter.NewFuzzy(true)

	// Use a configBufSize large enough to hold all lines in a single batch.
	// This ensures the fuzzy filter sees all lines at once and can sort globally.
	configBufSize := totalLines + 100
	fp := newFilterProcessor(f, query, configBufSize)

	// Set up pipeline: source -> filterProcessor -> destination
	p := pipeline.New()

	src := &sliceSource{lines: lines}
	p.SetSource(src)
	p.Add(fp)

	dst := NewMemoryBuffer(0)
	p.SetDestination(dst)

	ctx := f.NewContext(context.Background(), query)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := p.Run(ctx)
	require.NoError(t, err, "pipeline.Run should succeed")

	require.True(t, dst.Size() >= 2, "expected at least 2 matched lines, got %d", dst.Size())

	// The first result should be the exact match "exact" since it has the
	// longest continuous match and shortest line length.
	first, err := dst.LineAt(0)
	require.NoError(t, err)
	assert.Equal(t, "exact", first.DisplayString(), "best match should be first")
}

// sliceSource is a simple pipeline.Source backed by a slice of lines.
type sliceSource struct {
	lines []line.Line
}

func (s *sliceSource) Start(ctx context.Context, out pipeline.ChanOutput) {
	for _, l := range s.lines {
		select {
		case <-ctx.Done():
			return
		default:
			out.Send(ctx, l)
		}
	}
	out.SendEndMark(ctx, "end of sliceSource")
}

func (s *sliceSource) Reset() {}
