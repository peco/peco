package peco

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"context"

	"github.com/peco/peco/config"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/stretchr/testify/require"
)

func TestIssue212_SanityCheck(t *testing.T) {
	state, ctx := setupPecoTest(t)

	// Check if the default layout type is honored */
	// This the main issue on 212, but while we're at it, we're just
	// going to check that all the default values are as expected
	require.Equal(t, state.config.Layout, "top-down", "Default layout type should be 'top-down', got '%s'", state.config.Layout)
	require.Equal(t, len(state.config.Keymap), 0, "Default keymap should be empty, but got '%#v'", state.config.Keymap)

	defstyle := config.StyleSet{}
	defstyle.Init()
	require.Equal(t, state.config.Style, defstyle, "should be default style")
	require.Equal(t, state.config.Prompt, "QUERY>", "Default prompt should be 'QUERY>', but got '%s'", state.config.Prompt)

	// Okay, this time create a dummy config file, and read that in
	f, err := os.CreateTemp(t.TempDir(), "peco-test-config")
	require.NoError(t, err, "Failed to create temporary config file: %s", err)
	fn := f.Name()
	defer os.Remove(fn)

	io.WriteString(f, `{
    "Layout": "bottom-up"
}`)
	f.Close()

	state = newPeco()
	go state.Run(ctx)

	<-state.Ready()

	require.NoError(t, state.config.ReadFilename(fn), "Failed to read config: %s", err)
	require.Equal(t, state.config.Layout, "bottom-up", "Default layout type should be 'bottom-up', got '%s'", state.config.Layout)
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
	require.NoError(t, err, "newConfig should succeed")
	defer os.Remove(cfg)

	state := newPeco()
	state.configReader = defaultConfigReader
	require.NoError(t, state.config.Init(), "Config.Init should succeed")

	state.Argv = append(state.Argv, []string{"--rcfile", cfg}...)

	ctx, cancel := context.WithCancel(context.Background())
	go state.Run(ctx)
	defer cancel()

	<-state.Ready()

	ev := Event{
		Type: EventKey,
		Key:  keyseq.KeyCtrlT,
	}
	require.NoError(t, state.Keymap().ExecuteAction(ctx, state, ev), "ExecuteAction should succeed")

	// Brief pause to let async hub messages from the combined action
	// be processed before context cancellation tears down goroutines.
	time.Sleep(50 * time.Millisecond)
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
	lines = append(lines, line.NewRaw(0, "e_x_a_c_t filler text", false, false)) // fuzzy match
	for i := 1; i < totalLines-1; i++ {
		lines = append(lines, line.NewRaw(uint64(i), fmt.Sprintf("no match line %d", i), false, false))
	}
	lines = append(lines, line.NewRaw(uint64(totalLines-1), "exact", false, false)) // exact match, best result

	f := filter.NewFuzzy(true)

	// Use a configBufSize large enough to hold all lines in a single batch.
	// This ensures the fuzzy filter sees all lines at once and can sort globally.
	configBufSize := totalLines + 100
	fp := newFilterProcessor(f, query, configBufSize, nil)

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
	require.Equal(t, "exact", first.DisplayString(), "best match should be first")
}

// sliceSource is a simple pipeline.Source backed by a slice of lines.
type sliceSource struct {
	lines []line.Line
}

func (s *sliceSource) Start(ctx context.Context, out pipeline.ChanOutput) {
	defer close(out)
	for _, l := range s.lines {
		select {
		case <-ctx.Done():
			return
		default:
			out.Send(ctx, l)
		}
	}
}

func (s *sliceSource) Reset() {}
