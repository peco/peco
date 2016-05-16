package pipeline

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/context"
)

type RegexpFilter struct {
	OutputChannel
	rx *regexp.Regexp
}

func NewRegexpFilter(rx *regexp.Regexp) *RegexpFilter {
	return &RegexpFilter{
		OutputChannel: make(chan interface{}),
		rx:            rx,
	}
}

func (rf *RegexpFilter) Accept(ctx context.Context, p Producer) {
	defer fmt.Println("END RegexpFilter.Accept")
	defer rf.SendEndMark("end of RegexpFilter")
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-p.OutCh():
			if err, ok := v.(error); ok {
				if IsEndMark(err) {
					return
				}
			}

			if s, ok := v.(string); ok {
				if rf.rx.MatchString(s) {
					rf.Send(s)
				}
			}
		}
	}
}

type LineFeeder struct {
	OutputChannel
	lines []string
}

func NewLineFeeder(rdr io.Reader) *LineFeeder {
	scan := bufio.NewScanner(rdr)
	var lines []string
	for scan.Scan() {
		lines = append(lines, scan.Text())
	}
	return &LineFeeder{
		OutputChannel: make(chan interface{}),
		lines:         lines,
	}
}

func (f *LineFeeder) Start(ctx context.Context) {
	fmt.Println("START LineFeeder.Start")
	defer fmt.Println("END LineFeeder.Start")
	defer f.SendEndMark("end of LineFeeder")
	for _, s := range f.lines {
		f.Send(s)
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

func (r *Receiver) Accept(ctx context.Context, p Producer) {
	defer fmt.Println("END Receiver.Accept")
	defer close(r.done)

	for {
		select {
		case <-ctx.Done():
			return
		case v := <-p.OutCh():
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
	p.Source(src)
	p.Add(n1)
	p.Add(n2)
	p.Destination(dst)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p.Run(ctx)

	t.Logf("%#v", dst.lines)

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p.Run(ctx)
	t.Logf("%#v", dst.lines)
}