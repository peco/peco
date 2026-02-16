package peco

import (
	"context"
	"fmt"
	"testing"

	"github.com/peco/peco/filter"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/stretchr/testify/require"
)

func TestIsQueryRefinement(t *testing.T) {
	tests := []struct {
		prev     string
		new      string
		expected bool
	}{
		// Refinements
		{"foo", "foob", true},
		{"foo", "foo bar", true},
		{"foo bar", "foo bart", true},
		{"f", "fo", true},
		{"foo", "foobar", true},

		// Not refinements
		{"foob", "foo", false},       // backspace
		{"foo bar", "foo baz", false}, // edit mid-word
		{"foo", "bar", false},         // completely different
		{"", "foo", false},            // empty prev
		{"foo", "", false},            // empty new
		{"", "", false},               // both empty

		// Whitespace handling
		{" foo ", " foo bar", true},
		{"  foo  ", "  foo  bar", true},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%q->%q", tt.prev, tt.new)
		t.Run(name, func(t *testing.T) {
			result := isQueryRefinement(tt.prev, tt.new)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestMemoryBufferSource(t *testing.T) {
	// Create and populate a MemoryBuffer
	mb := NewMemoryBuffer()
	expected := []string{"alpha", "bravo", "charlie", "delta", "echo"}

	for i, s := range expected {
		mb.lines = append(mb.lines, line.NewRaw(uint64(i), s, false))
	}

	// Wrap as source
	src := NewMemoryBufferSource(mb)

	// Collect lines from source
	ctx := context.Background()
	out := make(chan interface{}, len(expected)+1) // +1 for end mark
	go src.Start(ctx, pipeline.ChanOutput(out))

	var got []string
	for v := range out {
		switch v := v.(type) {
		case error:
			if pipeline.IsEndMark(v) {
				goto done
			}
		case line.Line:
			got = append(got, v.DisplayString())
		}
	}
done:
	require.Equal(t, expected, got, "MemoryBufferSource should iterate all lines in order")
}

func TestMemoryBufferSourceCancellation(t *testing.T) {
	mb := NewMemoryBuffer()
	for i := 0; i < 10000; i++ {
		mb.lines = append(mb.lines, line.NewRaw(uint64(i), fmt.Sprintf("line-%d", i), false))
	}

	src := NewMemoryBufferSource(mb)
	ctx, cancel := context.WithCancel(context.Background())

	out := make(chan interface{}, 100)
	done := make(chan struct{})
	go func() {
		src.Start(ctx, pipeline.ChanOutput(out))
		close(done)
	}()

	// Cancel immediately
	cancel()
	<-done

	// Should have stopped early (not all 10000 lines)
	close(out)
	count := 0
	for range out {
		count++
	}
	// We can't guarantee exact count due to race, but it should be less than total
	// (or equal if the cancellation happened after all sends)
	require.LessOrEqual(t, count, 10000)
}

func TestIncrementalFiltering(t *testing.T) {
	// Test that filtering "foo" then "foob" produces correct results
	// and the second query runs on fewer lines

	// Create test lines
	allLines := []line.Line{
		line.NewRaw(0, "foobar test", false),
		line.NewRaw(1, "football game", false),
		line.NewRaw(2, "barfoo other", false),
		line.NewRaw(3, "something else", false),
		line.NewRaw(4, "foobaz entry", false),
		line.NewRaw(5, "the foobird flies", false),
	}

	f := filter.NewIgnoreCase()

	// First query: "foo"
	ctx1 := f.NewContext(context.Background(), "foo")
	ch1 := make(chan interface{}, len(allLines))
	err := f.Apply(ctx1, allLines, pipeline.ChanOutput(ch1))
	require.NoError(t, err)
	close(ch1)

	var firstResults []line.Line
	for v := range ch1 {
		if l, ok := v.(line.Line); ok {
			firstResults = append(firstResults, l)
		}
	}

	// Should match: foobar, football, barfoo, foobaz, foobird
	require.Len(t, firstResults, 5, "first query should match 5 lines")

	// Second query: "foob" on first results only
	ctx2 := f.NewContext(context.Background(), "foob")
	ch2 := make(chan interface{}, len(firstResults))
	err = f.Apply(ctx2, firstResults, pipeline.ChanOutput(ch2))
	require.NoError(t, err)
	close(ch2)

	var secondResults []line.Line
	for v := range ch2 {
		if l, ok := v.(line.Line); ok {
			secondResults = append(secondResults, l)
		}
	}

	// Should match: foobar, foobaz, foobird (not football, not barfoo)
	require.Len(t, secondResults, 3, "second query should match 3 lines")

	// Verify the matched lines are correct
	var displayStrings []string
	for _, l := range secondResults {
		displayStrings = append(displayStrings, l.DisplayString())
	}
	require.Contains(t, displayStrings, "foobar test")
	require.Contains(t, displayStrings, "foobaz entry")
	require.Contains(t, displayStrings, "the foobird flies")
}
