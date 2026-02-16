package filter

import (
	"context"
	"sync"
	"testing"

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
		case v, ok := <-out.OutCh():
			if !ok {
				return results
			}
			if l, ok := v.(line.Line); ok {
				results = append(results, l)
			}
		}
	}
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
		out := pipeline.ChanOutput(make(chan interface{}, 256))

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
		out := pipeline.ChanOutput(make(chan interface{}, 256))

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
		out := pipeline.ChanOutput(make(chan interface{}, 256))

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
