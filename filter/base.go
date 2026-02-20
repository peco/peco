package filter

import (
	"context"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// LineEmitter receives matched lines from a filter.
type LineEmitter interface {
	Emit(context.Context, line.Line)
}

// chanEmitter sends matched lines to a pipeline channel.
type chanEmitter struct {
	out pipeline.ChanOutput
}

func (e *chanEmitter) Emit(ctx context.Context, l line.Line) {
	_ = e.out.Send(ctx, l)
}

// LineCollector accumulates matched lines into a slice.
type LineCollector struct {
	lines []line.Line
}

// NewLineCollector creates a LineCollector pre-allocated with the given capacity.
func NewLineCollector(n int) *LineCollector {
	return &LineCollector{lines: make([]line.Line, 0, n)}
}

// Emit appends a matched line to the collector.
func (c *LineCollector) Emit(_ context.Context, l line.Line) {
	c.lines = append(c.lines, l)
}

// Lines returns the accumulated matched lines.
func (c *LineCollector) Lines() []line.Line {
	return c.lines
}

// baseFilter provides shared implementations of Apply, ApplyCollect,
// NewContext, and BufSize for filters that follow the applyInternal pattern.
// Filters embed this type and set applyFn to their type-specific matching logic.
type baseFilter struct {
	applyFn func(ctx context.Context, lines []line.Line, em LineEmitter) error
}

// NewContext returns a context initialized with the given query for pipeline use.
func (b *baseFilter) NewContext(ctx context.Context, query string) context.Context {
	return newContext(ctx, query)
}

func (b *baseFilter) BufSize() int {
	return 0
}

// Apply runs the filter's matching logic on lines, sending matches to out.
func (b *baseFilter) Apply(ctx context.Context, lines []line.Line, out pipeline.ChanOutput) error {
	return b.applyFn(ctx, lines, &chanEmitter{out: out})
}

// ApplyCollect runs the filter and returns matched lines directly as a slice,
// bypassing channel-based output for better performance in parallel paths.
func (b *baseFilter) ApplyCollect(ctx context.Context, lines []line.Line) ([]line.Line, error) {
	c := NewLineCollector(len(lines) / 2)
	err := b.applyFn(ctx, lines, c)
	return c.Lines(), err
}
