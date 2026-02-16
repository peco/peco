package filter

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/stretchr/testify/require"
)

type testIDGen struct {
	mu sync.Mutex
	id uint64
}

func (g *testIDGen) Next() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.id++
	return g.id
}

// collectOutput drains the output channel and returns the lines sent to it.
func collectOutput(ctx context.Context, out pipeline.ChanOutput) []line.Line {
	var results []line.Line
	for {
		select {
		case <-ctx.Done():
			return results
		case l, ok := <-out.OutCh():
			if !ok {
				return results
			}
			results = append(results, l)
		}
	}
}

func TestExternalCmd_CancelCleansUpGoroutine(t *testing.T) {
	idgen := &testIDGen{}

	lines := []line.Line{
		line.NewRaw(idgen.Next(), "hello", false, false),
	}

	// Use "cat" which reads stdin and writes to stdout; it will block
	// waiting for EOF on stdin, but since we provide input via a buffer
	// it completes quickly. Instead, use "sleep" which blocks for a long time.
	ecf := NewExternalCmd("sleep", "sleep", []string{"60"}, 0, idgen, false)
	ctx, cancel := context.WithCancel(ecf.NewContext(context.Background(), "test"))
	out := pipeline.ChanOutput(make(chan line.Line, 256))

	// Record goroutine count before Apply
	runtime.GC()
	before := runtime.NumGoroutine()

	done := make(chan error, 1)
	go func() {
		done <- ecf.Apply(ctx, lines, out)
	}()

	// Give the command time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Apply should return promptly
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Apply did not return after context cancellation")
	}

	// Wait briefly for goroutine cleanup, then verify no goroutine leak
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		after := runtime.NumGoroutine()
		// Allow some slack (test infrastructure goroutines)
		if after <= before+1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("goroutine leak: before=%d, after=%d", before, runtime.NumGoroutine())
}

func TestExternalCmd_ApplyPanicReturnsError(t *testing.T) {
	idgen := &testIDGen{}

	lines := []line.Line{
		line.NewRaw(idgen.Next(), "hello", false, false),
	}

	ecf := NewExternalCmd("cat", "cat", nil, 0, idgen, false)
	out := pipeline.ChanOutput(make(chan line.Line, 256))

	// Call Apply with a context that does NOT have the query key set.
	// This triggers a nil interface type assertion panic at the line:
	//   query := ctx.Value(queryKey).(string)
	// The bug: this panic was silently swallowed, returning nil error.
	err := ecf.Apply(context.Background(), lines, out)
	require.Error(t, err, "Apply should return an error when an internal panic occurs, not swallow it silently")
	require.Contains(t, err.Error(), "panic")
}

func TestExternalCmdFilter_NullSep(t *testing.T) {
	t.Run("preserves Output with enableSep", func(t *testing.T) {
		idgen := &testIDGen{}

		// Create lines with null separator: display\0output
		lines := []line.Line{
			line.NewRaw(idgen.Next(), "apple\x00/fruit/apple", true, false),
			line.NewRaw(idgen.Next(), "banana\x00/fruit/banana", true, false),
			line.NewRaw(idgen.Next(), "apricot\x00/fruit/apricot", true, false),
		}

		// Verify the lines are set up correctly
		require.Equal(t, "apple", lines[0].DisplayString())
		require.Equal(t, "/fruit/apple", lines[0].Output())

		// Use grep to filter for lines containing "ap" (matches apple, apricot)
		ecf := NewExternalCmd("grep", "grep", []string{"ap"}, 0, idgen, true)

		ctx := ecf.NewContext(context.Background(), "ap")
		out := pipeline.ChanOutput(make(chan line.Line, 256))

		var results []line.Line
		done := make(chan struct{})
		go func() {
			defer close(done)
			results = collectOutput(ctx, out)
		}()

		err := ecf.Apply(ctx, lines, out)
		require.NoError(t, err)
		close(out)
		<-done

		require.Len(t, results, 2)

		// The key assertion: Output() must return the post-separator text,
		// not the display text that the external filter matched on.
		require.Equal(t, "apple", results[0].DisplayString())
		require.Equal(t, "/fruit/apple", results[0].Output())

		require.Equal(t, "apricot", results[1].DisplayString())
		require.Equal(t, "/fruit/apricot", results[1].Output())
	})

	t.Run("without enableSep creates new lines", func(t *testing.T) {
		idgen := &testIDGen{}

		lines := []line.Line{
			line.NewRaw(idgen.Next(), "apple", false, false),
			line.NewRaw(idgen.Next(), "banana", false, false),
			line.NewRaw(idgen.Next(), "apricot", false, false),
		}

		ecf := NewExternalCmd("grep", "grep", []string{"ap"}, 0, idgen, false)

		ctx := ecf.NewContext(context.Background(), "ap")
		out := pipeline.ChanOutput(make(chan line.Line, 256))

		var results []line.Line
		done := make(chan struct{})
		go func() {
			defer close(done)
			results = collectOutput(ctx, out)
		}()

		err := ecf.Apply(ctx, lines, out)
		require.NoError(t, err)
		close(out)
		<-done

		require.Len(t, results, 2)
		require.Equal(t, "apple", results[0].DisplayString())
		require.Equal(t, "apple", results[0].Output())
		require.Equal(t, "apricot", results[1].DisplayString())
		require.Equal(t, "apricot", results[1].Output())
	})

	t.Run("duplicate display strings preserve order", func(t *testing.T) {
		idgen := &testIDGen{}

		// Three lines with the same display text but different outputs
		lines := []line.Line{
			line.NewRaw(idgen.Next(), "dup\x00first", true, false),
			line.NewRaw(idgen.Next(), "dup\x00second", true, false),
			line.NewRaw(idgen.Next(), "dup\x00third", true, false),
		}

		// Use grep to return all lines (match literal "dup")
		ecf := NewExternalCmd("grep", "grep", []string{"-F", "$QUERY"}, 0, idgen, true)

		ctx := ecf.NewContext(context.Background(), "dup")
		out := pipeline.ChanOutput(make(chan line.Line, 256))

		var results []line.Line
		done := make(chan struct{})
		go func() {
			defer close(done)
			results = collectOutput(ctx, out)
		}()

		err := ecf.Apply(ctx, lines, out)
		require.NoError(t, err)
		close(out)
		<-done

		require.Len(t, results, 3)
		require.Equal(t, "first", results[0].Output())
		require.Equal(t, "second", results[1].Output())
		require.Equal(t, "third", results[2].Output())
	})
}
