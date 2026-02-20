package peco

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
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
		{"foob", "foo", false},        // backspace
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

func TestIsQueryRefinementWithNegation(t *testing.T) {
	tests := []struct {
		prev     string
		new      string
		expected bool
	}{
		// Adding a negative term is a refinement (narrows results)
		{"foo", "foo -bar", true},
		// Adding more negative terms is still a refinement
		{"foo -bar", "foo -bar -baz", true},
		// Removing a negative term is NOT a refinement (widens results)
		{"foo -bar -baz", "foo -bar", false},
		// Extending positive while keeping negatives
		{"foo -bar", "fooX -bar", true},
		// All-negative refinement
		{"-foo", "-foo -bar", true},
		// Changing a negative term is not a refinement
		{"-foo", "-bar", false},
		// Adding positive to all-negative is a refinement
		{"-foo", "hello -foo", true},
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
	mb := NewMemoryBuffer(0)
	expected := []string{"alpha", "bravo", "charlie", "delta", "echo"}

	for i, s := range expected {
		mb.lines = append(mb.lines, line.NewRaw(uint64(i), s, false, false))
	}

	// Wrap as source
	src := NewMemoryBufferSource(mb)

	// Collect lines from source
	ctx := context.Background()
	out := make(chan line.Line, len(expected))
	go src.Start(ctx, pipeline.ChanOutput(out))

	var got []string
	for l := range out {
		got = append(got, l.DisplayString())
	}
	require.Equal(t, expected, got, "MemoryBufferSource should iterate all lines in order")
}

func TestMemoryBufferSourceCancellation(t *testing.T) {
	mb := NewMemoryBuffer(0)
	for i := range 10000 {
		mb.lines = append(mb.lines, line.NewRaw(uint64(i), fmt.Sprintf("line-%d", i), false, false))
	}

	src := NewMemoryBufferSource(mb)
	ctx, cancel := context.WithCancel(context.Background())

	out := make(chan line.Line, 100)
	done := make(chan struct{})
	go func() {
		src.Start(ctx, pipeline.ChanOutput(out))
		close(done)
	}()

	// Cancel immediately
	cancel()
	<-done

	// Source closed the channel; drain to count lines sent
	count := 0
	for range out {
		count++
	}
	// We can't guarantee exact count due to race, but it should be less than total
	// (or equal if the cancellation happened after all sends)
	require.LessOrEqual(t, count, 10000)
}

// TestAcceptAndFilterSerial exercises the serial (non-parallel) batching path
// through AcceptAndFilter. It sends lines through a channel and verifies
// that the filter is applied correctly and all matching lines appear in the output.
func TestAcceptAndFilterSerial(t *testing.T) {
	f := filter.NewFuzzy(true) // Fuzzy with sortLongest=true does NOT support parallel
	require.False(t, f.SupportsParallel(), "Fuzzy with sortLongest should not support parallel")

	inputLines := []line.Line{
		line.NewRaw(0, "alpha bravo", false, false),
		line.NewRaw(1, "charlie delta", false, false),
		line.NewRaw(2, "alpha charlie", false, false),
		line.NewRaw(3, "echo foxtrot", false, false),
		line.NewRaw(4, "alpha delta", false, false),
	}

	ctx := f.NewContext(context.Background(), "alpha")
	in := make(chan line.Line, len(inputLines))
	for _, l := range inputLines {
		in <- l
	}
	close(in)

	out := make(chan line.Line, len(inputLines))
	AcceptAndFilter(ctx, f, 0, in, pipeline.ChanOutput(out))

	var got []string
	for l := range out {
		got = append(got, l.DisplayString())
	}

	// Fuzzy with sortLongest sorts by: longer match > earlier match > shorter line.
	// All three match "alpha" at position 0 with length 5, so the tiebreaker is
	// line length (shorter first). "alpha bravo" and "alpha delta" tie at 11 chars,
	// so sort.SliceStable preserves their input order.
	require.Equal(t, []string{"alpha bravo", "alpha delta", "alpha charlie"}, got)
}

// TestAcceptAndFilterParallel exercises the parallel batching path.
// IgnoreCase (a Regexp filter) supports parallel execution.
func TestAcceptAndFilterParallel(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("parallel path requires GOMAXPROCS >= 2")
	}

	f := filter.NewIgnoreCase()
	require.True(t, f.SupportsParallel(), "IgnoreCase should support parallel")

	inputLines := []line.Line{
		line.NewRaw(0, "foobar test", false, false),
		line.NewRaw(1, "football game", false, false),
		line.NewRaw(2, "something else", false, false),
		line.NewRaw(3, "foobaz entry", false, false),
		line.NewRaw(4, "barfoo other", false, false),
		line.NewRaw(5, "the foobird flies", false, false),
		line.NewRaw(6, "no match here", false, false),
	}

	ctx := f.NewContext(context.Background(), "foo")
	in := make(chan line.Line, len(inputLines))
	for _, l := range inputLines {
		in <- l
	}
	close(in)

	out := make(chan line.Line, len(inputLines))
	AcceptAndFilter(ctx, f, 0, in, pipeline.ChanOutput(out))

	var got []string
	for l := range out {
		got = append(got, l.DisplayString())
	}

	// Parallel preserves input order via ordered chunks — verify exact order
	expected := []string{"foobar test", "football game", "foobaz entry", "barfoo other", "the foobird flies"}
	require.Equal(t, expected, got)
}

func TestIncrementalFiltering(t *testing.T) {
	// Test that filtering "foo" then "foob" produces correct results
	// and the second query runs on fewer lines

	// Create test lines
	allLines := []line.Line{
		line.NewRaw(0, "foobar test", false, false),
		line.NewRaw(1, "football game", false, false),
		line.NewRaw(2, "barfoo other", false, false),
		line.NewRaw(3, "something else", false, false),
		line.NewRaw(4, "foobaz entry", false, false),
		line.NewRaw(5, "the foobird flies", false, false),
	}

	f := filter.NewIgnoreCase()

	// First query: "foo"
	ctx1 := f.NewContext(context.Background(), "foo")
	ch1 := make(chan line.Line, len(allLines))
	err := f.Apply(ctx1, allLines, pipeline.ChanOutput(ch1))
	require.NoError(t, err)
	close(ch1)

	var firstResults []line.Line
	for l := range ch1 {
		firstResults = append(firstResults, l)
	}

	// Should match: foobar, football, barfoo, foobaz, foobird
	require.Len(t, firstResults, 5, "first query should match 5 lines")

	// Second query: "foob" on first results only
	ctx2 := f.NewContext(context.Background(), "foob")
	ch2 := make(chan line.Line, len(firstResults))
	err = f.Apply(ctx2, firstResults, pipeline.ChanOutput(ch2))
	require.NoError(t, err)
	close(ch2)

	var secondResults []line.Line
	for l := range ch2 {
		secondResults = append(secondResults, l)
	}

	// Should match: foobar, foobaz, foobird (not football, not barfoo)
	require.Len(t, secondResults, 3, "second query should match 3 lines")

	// Verify the matched lines are correct
	displayStrings := make([]string, 0, len(secondResults))
	for _, l := range secondResults {
		displayStrings = append(displayStrings, l.DisplayString())
	}
	require.Contains(t, displayStrings, "foobar test")
	require.Contains(t, displayStrings, "foobaz entry")
	require.Contains(t, displayStrings, "the foobird flies")
}

// TestFrozenCacheInvalidation verifies that the incremental filter cache
// is invalidated when the frozen state changes. Without this fix, cached
// results from the original source would be reused after freezing, producing
// wrong results.
func TestFrozenCacheInvalidation(t *testing.T) {
	// Set up a Peco state with a recording hub
	state := New()
	rHub := &recordingHub{}
	state.hub = rHub
	state.selection = NewSelection()

	// Populate filters (needed by Filter.Work)
	state.filters.Add(filter.NewIgnoreCase())

	// Create original source with lines that include "apache" matches
	src := &Source{}
	src.setupDone = make(chan struct{})
	for i, s := range []string{"apache config", "apache error", "APACHE LOG", "nginx proxy", "redis cache"} {
		src.Append(line.NewRaw(uint64(i), s, false, false))
	}
	close(src.setupDone)
	state.source = src
	state.currentLineBuffer = src

	// Create a Filter and simulate a previous "apache" query
	f := NewFilter(state)

	// Run a real filter for "apache" to populate the cache
	q1 := hub.NewPayload("apache", false)
	f.Work(context.Background(), q1)

	// Verify cache was populated from original source (3 matches: apache config, apache error, APACHE LOG)
	f.prevMu.Lock()
	require.Equal(t, "apache", f.prevQuery)
	require.NotNil(t, f.prevResults)
	require.Equal(t, 3, f.prevResults.Size(), "should have 3 'apache' matches from original source")
	require.Nil(t, f.prevFrozenSrc, "prevFrozenSrc should be nil (not frozen)")
	f.prevMu.Unlock()

	// Now freeze with a subset: only "apache config" and "nginx proxy"
	frozen := NewMemoryBuffer(0)
	frozen.AppendLine(line.NewRaw(0, "apache config", false, false))
	frozen.AppendLine(line.NewRaw(3, "nginx proxy", false, false))
	frozen.MarkComplete()
	state.Frozen().Set(frozen)

	// Query "apache c" — this is a refinement of "apache", so without the fix
	// the stale cache (3 results from original source) would be used instead
	// of filtering from the frozen buffer (which has only 1 apache match).
	q2 := hub.NewPayload("apache c", false)
	f.Work(context.Background(), q2)

	// The result should come from the frozen buffer, not the stale cache.
	// Frozen buffer has "apache config" and "nginx proxy". Filtering for
	// "apache c" should match only "apache config".
	buf := state.CurrentLineBuffer()
	require.Equal(t, 1, buf.Size(), "should have 1 match from frozen buffer")
	l, err := buf.LineAt(0)
	require.NoError(t, err)
	require.Equal(t, "apache config", l.Buffer())

	// Verify cache is now updated with frozen source
	f.prevMu.Lock()
	require.Equal(t, frozen, f.prevFrozenSrc, "prevFrozenSrc should match current frozen source")
	f.prevMu.Unlock()
}

// errorFilter is a mock filter that always returns an error from Apply.
type errorFilter struct {
	err error
}

func (f *errorFilter) Apply(_ context.Context, _ []line.Line, _ pipeline.ChanOutput) error {
	return f.err
}

func (f *errorFilter) BufSize() int                                             { return 0 }
func (f *errorFilter) NewContext(ctx context.Context, _ string) context.Context { return ctx }
func (f *errorFilter) String() string                                           { return "error-filter" }
func (f *errorFilter) SupportsParallel() bool                                   { return false }

// TestFilterApplyErrorReporting verifies that when a filter's Apply method
// returns an error, the error is propagated to the onError callback (which in
// production sends a status bar message to the user).
func TestFilterApplyErrorReporting(t *testing.T) {
	simulatedErr := errors.New("simulated filter error")
	ef := &errorFilter{err: simulatedErr}

	inputLines := []line.Line{
		line.NewRaw(0, "alpha", false, false),
		line.NewRaw(1, "bravo", false, false),
	}

	in := make(chan line.Line, len(inputLines))
	for _, l := range inputLines {
		in <- l
	}
	close(in)

	var mu sync.Mutex
	var reported []error
	onError := FilterErrorHandlerFunc(func(err error) {
		mu.Lock()
		defer mu.Unlock()
		reported = append(reported, err)
	})

	out := make(chan line.Line, len(inputLines))
	acceptAndFilter(context.Background(), ef, 0, onError, in, pipeline.ChanOutput(out))

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, reported, 1, "error handler should have been called once")
	require.Equal(t, simulatedErr, reported[0])
}
