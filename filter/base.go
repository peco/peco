package filter

import (
	"context"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// baseFilter provides shared implementations of Apply, ApplyCollect,
// NewContext, and BufSize for filters that follow the applyInternal pattern.
// Filters embed this type and set applyFn to their type-specific matching logic.
type baseFilter struct {
	applyFn func(ctx context.Context, lines []line.Line, emit func(line.Line)) error
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
	return b.applyFn(ctx, lines, func(l line.Line) {
		_ = out.Send(ctx, l)
	})
}

// ApplyCollect runs the filter and returns matched lines directly as a slice,
// bypassing channel-based output for better performance in parallel paths.
func (b *baseFilter) ApplyCollect(ctx context.Context, lines []line.Line) ([]line.Line, error) {
	result := make([]line.Line, 0, len(lines)/2)
	err := b.applyFn(ctx, lines, func(l line.Line) {
		result = append(result, l)
	})
	return result, err
}
