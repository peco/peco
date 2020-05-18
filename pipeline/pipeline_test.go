package pipeline

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"context"

	"github.com/stretchr/testify/assert"
)

type RegexpFilter struct {
	rx *regexp.Regexp
}

func NewRegexpFilter(rx *regexp.Regexp) *RegexpFilter {
	return &RegexpFilter{
		rx: rx,
	}
}

func (rf *RegexpFilter) Accept(ctx context.Context, in chan interface{}, out ChanOutput) {
	defer func() { _ = out.SendEndMark("end of RegexpFilter") }()
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-in:
			if err, ok := v.(error); ok {
				if IsEndMark(err) {
					return
				}
			}

			if s, ok := v.(string); ok {
				if rf.rx.MatchString(s) {
					_ = out.Send(s)
				}
			}
		}
	}
}

type LineFeeder struct {
	lines []string
}

func NewLineFeeder(rdr io.Reader) *LineFeeder {
	scan := bufio.NewScanner(rdr)
	var lines []string
	for scan.Scan() {
		lines = append(lines, scan.Text())
	}
	return &LineFeeder{
		lines: lines,
	}
}

func (f *LineFeeder) Reset() {
}

func (f *LineFeeder) Start(ctx context.Context, out ChanOutput) {
	defer func() { _ = out.SendEndMark("end of LineFeeder") }()
	for _, s := range f.lines {
		_ = out.Send(s)
	}
}

type Receiver struct {
	lines []string
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

func (r *Receiver) Accept(ctx context.Context, in chan interface{}, out ChanOutput) {
	defer close(r.done)

	for {
		select {
		case <-ctx.Done():
			return
		case v := <-in:
			if err, ok := v.(error); ok {
				if IsEndMark(err) {
					return
				}
			}

			if s, ok := v.(string); ok {
				r.lines = append(r.lines, s)
			}
		}
	}
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
	if !assert.NoError(t, p.Run(ctx), "p.Run exits with no error") {
		return
	}
	t.Logf("%#v", dst.lines)
}
