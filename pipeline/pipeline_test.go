package pipeline

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"context"

	"github.com/peco/peco/line"
	"github.com/stretchr/testify/require"
)

type RegexpFilter struct {
	rx *regexp.Regexp
}

func NewRegexpFilter(rx *regexp.Regexp) *RegexpFilter {
	return &RegexpFilter{
		rx: rx,
	}
}

func (rf *RegexpFilter) Accept(ctx context.Context, in <-chan line.Line, out ChanOutput) {
	defer close(out)
	for {
		select {
		case <-ctx.Done():
			return
		case v, ok := <-in:
			if !ok {
				return
			}
			if rf.rx.MatchString(v.DisplayString()) {
				out.Send(ctx, v)
			}
		}
	}
}

type LineFeeder struct {
	lines []line.Line
}

func NewLineFeeder(rdr io.Reader) *LineFeeder {
	scan := bufio.NewScanner(rdr)
	var lines []line.Line
	var id uint64
	for scan.Scan() {
		lines = append(lines, line.NewRaw(id, scan.Text(), false, false))
		id++
	}
	return &LineFeeder{
		lines: lines,
	}
}

func (f *LineFeeder) Reset() {
}

func (f *LineFeeder) Start(ctx context.Context, out ChanOutput) {
	defer close(out)
	for _, l := range f.lines {
		out.Send(ctx, l)
	}
}

type Receiver struct {
	lines []line.Line
	done  chan struct{}
}

func NewReceiver() *Receiver {
	r := &Receiver{}
	r.Reset()
	return r
}

func (r *Receiver) Reset() {
	r.done = make(chan struct{})
	r.lines = nil
}

func (r *Receiver) Done() <-chan struct{} {
	return r.done
}

func (r *Receiver) Accept(ctx context.Context, in <-chan line.Line, _ ChanOutput) {
	defer close(r.done)

	for {
		select {
		case <-ctx.Done():
			return
		case v, ok := <-in:
			if !ok {
				return
			}
			r.lines = append(r.lines, v)
		}
	}
}

func TestQueryContext(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		ctx := NewQueryContext(context.Background(), "hello")
		got := QueryFromContext(ctx)
		if got != "hello" {
			t.Fatalf("expected %q, got %q", "hello", got)
		}
	})

	t.Run("missing key returns empty", func(t *testing.T) {
		got := QueryFromContext(context.Background())
		if got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}
	})
}

func TestPipeline(t *testing.T) {
	src := NewLineFeeder(strings.NewReader(`foo
bar
foobar
barfoo
`))
	n1 := NewRegexpFilter(regexp.MustCompile(`^foo`))
	n2 := NewRegexpFilter(regexp.MustCompile(`bar$`))
	dst := NewReceiver()

	p := New()
	p.SetSource(src)
	p.Add(n1)
	p.Add(n2)
	p.SetDestination(dst)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p.Run(ctx)

	got := make([]string, 0, len(dst.lines))
	for _, l := range dst.lines {
		got = append(got, l.DisplayString())
	}
	require.Equal(t, []string{"foobar"}, got)
}
