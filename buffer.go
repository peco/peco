package peco

import (
	"time"

	"context"

	"github.com/lestrrat/go-pdebug"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/pkg/errors"
)

func NewFilteredBuffer(src Buffer, page, perPage int) *FilteredBuffer {
	fb := FilteredBuffer{
		src: src,
	}

	s := perPage * (page - 1)
	if s > src.Size() {
		return &fb
	}

	selection := make([]int, 0, src.Size())
	e := s + perPage
	if e >= src.Size() {
		e = src.Size()
	}

	for i := s; i < e; i++ {
		selection = append(selection, i)
	}
	fb.selection = selection

	return &fb
}

func (flb *FilteredBuffer) Append(l line.Line) (line.Line, error) {
	return l, nil
}

// LineAt returns the line at index `i`. Note that the i-th element
// in this filtered buffer may actually correspond to a totally
// different line number in the source buffer.
func (flb FilteredBuffer) LineAt(i int) (line.Line, error) {
	if i >= len(flb.selection) {
		return nil, errors.Errorf("specified index %d is out of range", len(flb.selection))
	}
	return flb.src.LineAt(flb.selection[i])
}

// Size returns the number of lines in the buffer
func (flb FilteredBuffer) Size() int {
	return len(flb.selection)
}

func NewMemoryBuffer() *MemoryBuffer {
	mb := &MemoryBuffer{}
	mb.Reset()
	return mb
}

func (mb *MemoryBuffer) Append(l line.Line) {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	bufferAppend(&mb.lines, l)
}

func bufferAppend(lines *[]line.Line, l line.Line) {
	*lines = append(*lines, l)
}

func (mb *MemoryBuffer) Size() int {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return bufferSize(mb.lines)
}

func bufferSize(lines []line.Line) int {
	return len(lines)
}

func (mb *MemoryBuffer) Reset() {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	if pdebug.Enabled {
		g := pdebug.Marker("MemoryBuffer.Reset")
		defer g.End()
	}
	mb.done = make(chan struct{})
	mb.lines = []line.Line(nil)
}

func (mb *MemoryBuffer) Done() <-chan struct{} {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.done
}

func (mb *MemoryBuffer) Accept(ctx context.Context, in chan interface{}, _ pipeline.ChanOutput) {
	if pdebug.Enabled {
		g := pdebug.Marker("MemoryBuffer.Accept")
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
				pdebug.Printf("MemoryBuffer received context done")
			}
			return
		case v := <-in:
			switch v.(type) {
			case error:
				if pipeline.IsEndMark(v.(error)) {
					if pdebug.Enabled {
						pdebug.Printf("MemoryBuffer received end mark (read %d lines, %s since starting accept loop)", len(mb.lines), time.Since(start).String())
					}
					return
				}
			case line.Line:
				mb.mutex.Lock()
				mb.lines = append(mb.lines, v.(line.Line))
				mb.mutex.Unlock()
			}
		}
	}
}

func (mb *MemoryBuffer) LineAt(n int) (line.Line, error) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return bufferLineAt(mb.lines, n)
}

func bufferLineAt(lines []line.Line, n int) (line.Line, error) {
	if s := len(lines); s <= 0 || n >= s {
		return nil, errors.New("empty buffer")
	}

	return lines[n], nil
}
