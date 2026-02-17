package filter

import (
	"context"

	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

func (b *baseFilter) NewContext(ctx context.Context, query string) context.Context {
	return newContext(ctx, query)
}

func (b *baseFilter) BufSize() int {
	return 0
}

func (b *baseFilter) Apply(ctx context.Context, lines []line.Line, out pipeline.ChanOutput) error {
	return b.applyFn(ctx, lines, func(l line.Line) {
		_ = out.Send(ctx, l)
	})
}

func (b *baseFilter) ApplyCollect(ctx context.Context, lines []line.Line) ([]line.Line, error) {
	result := make([]line.Line, 0, len(lines)/2)
	err := b.applyFn(ctx, lines, func(l line.Line) {
		result = append(result, l)
	})
	return result, err
}
