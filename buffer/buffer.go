package buffer

import (
	"time"

	"context"

	"github.com/lestrrat-go/pdebug/v2"
	runewidth "github.com/mattn/go-runewidth"
	"github.com/peco/peco/internal/location"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/pkg/errors"
)

func Crop(src Buffer, loc *location.Location) *Filtered {
	return NewFiltered(src, loc.Page(), loc.PerPage())
}

func NewFiltered(src Buffer, page, perPage int) *Filtered {
	fb := Filtered{
		src: src,
	}

	start := perPage * (page - 1)

	// if for whatever reason we wanted a page that goes over the
	// capacity of the original buffer, we don't need to do any more
	// calculations. bail out
	if start > src.Size() {
		return &fb
	}

	// Copy over the selections that are applicable to this filtered buffer.
	selection := make([]int, 0, src.Size())
	end := start + perPage
	if end >= src.Size() {
		end = src.Size()
	}

	lines := src.LinesInRange(start, end)
	var maxcols int
	for i := start; i < end; i++ {
		selection = append(selection, i)
		cols := runewidth.StringWidth(lines[i-start].DisplayString())
		if cols > maxcols {
			maxcols = cols
		}
	}
	fb.selection = selection
	fb.maxcols = maxcols

	return &fb
}

func (flb *Filtered) LinesInRange(_ int, _ int) []line.Line {
	panic("unimplemented")
}

// MaxColumn returns the max column size, which controls the amount we
// can scroll to the right
func (flb *Filtered) MaxColumn() int {
	return flb.maxcols
}

// LineAt returns the line at index `i`. Note that the i-th element
// in this filtered buffer may actually correspond to a totally
// different line number in the source buffer.
func (flb Filtered) LineAt(i int) (line.Line, error) {
	if i >= len(flb.selection) {
		return nil, errors.Errorf("specified index %d is out of range", len(flb.selection))
	}
	return flb.src.LineAt(flb.selection[i])
}

// Size returns the number of lines in the buffer
func (flb Filtered) Size() int {
	return len(flb.selection)
}

func NewMemory() *Memory {
	mb := &Memory{}
	mb.Reset()
	return mb
}

func (mb *Memory) Size() int {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return len(mb.lines)
}

func (mb *Memory) Reset() {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	if pdebug.Enabled {
		g := pdebug.Marker(context.TODO(), "Memory.Reset")
		defer g.End()
	}
	mb.done = make(chan struct{})
	mb.lines = []line.Line(nil)
}

func (mb *Memory) Done() <-chan struct{} {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.done
}

func (mb *Memory) Accept(ctx context.Context, in chan interface{}, _ pipeline.ChanOutput) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "Memory.Accept")
		defer g.End()
	}
	defer func() {
		mb.mutex.Lock()
		close(mb.done)
		mb.mutex.Unlock()
	}()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf(ctx, "Memory received context done")
			}
			return
		case v := <-in:
			switch v := v.(type) {
			case error:
				if pipeline.IsEndMark(v) {
					if pdebug.Enabled {
						pdebug.Printf(ctx, "Memory received end mark (read %d lines, %s since starting accept loop)", len(mb.lines), time.Since(start).String())
					}
					return
				}
			case line.Line:
				mb.mutex.Lock()
				mb.lines = append(mb.lines, v)
				mb.mutex.Unlock()
			}
		}
	}
}

func (mb *Memory) LineAt(n int) (line.Line, error) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return LineAt(mb.lines, n)
}

func (mb *Memory) LinesInRange(start, end int) []line.Line {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.lines[start:end]
}

func LineAt(lines []line.Line, n int) (line.Line, error) {
	// TODO: This code smells
	if s := len(lines); s <= 0 || n >= s {
		return nil, errors.New("empty buffer")
	}

	return lines[n], nil
}
