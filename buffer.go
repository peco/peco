package peco

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"context"

	"github.com/lestrrat-go/pdebug"
	runewidth "github.com/mattn/go-runewidth"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// Buffer interface is used for containers for lines to be
// processed by peco. The unexported linesInRange method seals this
// interface to the peco package — external packages cannot implement it.
// This is intentional: linesInRange is an internal optimization for
// efficient pagination in NewFilteredBuffer, not part of the public contract.
type Buffer interface {
	linesInRange(int, int) []line.Line
	LineAt(int) (line.Line, error)
	Size() int
}

// FilteredBuffer holds a "filtered" buffer. It holds a reference to
// the source buffer (note: should be immutable) and a list of indices
// into the source buffer
type FilteredBuffer struct {
	maxcols   int
	src       Buffer
	selection []int // maps from our index to src's index
}

// MemoryBuffer is an implementation of Buffer
type MemoryBuffer struct {
	done     chan struct{}
	doneOnce sync.Once
	lines    []line.Line
	mutex    sync.RWMutex
}

// ContextLine wraps a line.Line to mark it as a context line (non-matched
// surrounding line shown during ZoomIn). Detected via type assertion in
// ListArea.Draw() to apply the Context style.
type ContextLine struct {
	line.Line
}

// NewFilteredBuffer creates a FilteredBuffer containing one page of lines from
// the source buffer, computing the maximum column width for horizontal scrolling.
func NewFilteredBuffer(src Buffer, page, perPage int) *FilteredBuffer {
	fb := FilteredBuffer{
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
	end := min(start+perPage, src.Size())
	selection := make([]int, 0, end-start)

	lines := src.linesInRange(start, end)
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

// MaxColumn returns the max column size, which controls the amount we
// can scroll to the right
func (flb *FilteredBuffer) MaxColumn() int {
	return flb.maxcols
}

// LineAt returns the line at index `i`. Note that the i-th element
// in this filtered buffer may actually correspond to a totally
// different line number in the source buffer.
func (flb FilteredBuffer) LineAt(i int) (line.Line, error) {
	if i < 0 || i >= len(flb.selection) {
		return nil, fmt.Errorf("specified index %d is out of range (size=%d)", i, len(flb.selection))
	}
	return flb.src.LineAt(flb.selection[i])
}

// Size returns the number of lines in the buffer
func (flb FilteredBuffer) Size() int {
	return len(flb.selection)
}

const defaultMemoryBufferCap = 1024

// NewMemoryBuffer creates a new MemoryBuffer. If cap > 0, the lines
// slice is pre-allocated with that capacity; otherwise it defaults to
// defaultMemoryBufferCap.
func NewMemoryBuffer(capacity int) *MemoryBuffer {
	if capacity <= 0 {
		capacity = defaultMemoryBufferCap
	}
	mb := &MemoryBuffer{}
	mb.done = make(chan struct{})
	mb.lines = make([]line.Line, 0, capacity)
	return mb
}

// Size returns the number of lines currently held in the buffer, thread-safe.
func (mb *MemoryBuffer) Size() int {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return len(mb.lines)
}

// Reset clears the buffer, reinitializing the done channel and lines slice
// so the buffer can be reused for a new pipeline run.
func (mb *MemoryBuffer) Reset() {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()
	if pdebug.Enabled {
		g := pdebug.Marker("MemoryBuffer.Reset")
		defer g.End()
	}
	// Return pooled Matched objects before discarding the slice
	for _, l := range mb.lines {
		if m, ok := l.(*line.Matched); ok {
			line.ReleaseMatched(m)
		}
	}
	mb.done = make(chan struct{})
	mb.doneOnce = sync.Once{}
	mb.lines = []line.Line(nil)
}

// MarkComplete signals that the buffer is fully populated. It is safe
// to call multiple times; only the first call closes the done channel.
// Use this instead of manually closing the done channel when populating
// a MemoryBuffer outside of the pipeline (e.g. freeze).
func (mb *MemoryBuffer) MarkComplete() {
	mb.doneOnce.Do(func() {
		mb.mutex.Lock()
		close(mb.done)
		mb.mutex.Unlock()
	})
}

// Done returns a channel that is closed when the buffer has been fully populated.
func (mb *MemoryBuffer) Done() <-chan struct{} {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.done
}

// Accept receives lines from a pipeline input channel and appends them to the
// buffer in batches. It marks the buffer complete when the channel closes or
// the context is cancelled.
func (mb *MemoryBuffer) Accept(ctx context.Context, in <-chan line.Line, _ pipeline.ChanOutput) {
	if pdebug.Enabled {
		g := pdebug.Marker("MemoryBuffer.Accept")
		defer g.End()
	}
	defer mb.MarkComplete()

	// batch collects lines from the channel so we can append them
	// under a single lock acquisition instead of locking per line.
	batch := make([]line.Line, 0, 256)

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("MemoryBuffer received context done")
			}
			return
		case v, ok := <-in:
			if !ok {
				if pdebug.Enabled {
					pdebug.Printf("MemoryBuffer input channel closed (read %d lines, %s since starting accept loop)", len(mb.lines)+len(batch), time.Since(start).String())
				}
				// Flush remaining batch
				if len(batch) > 0 {
					mb.mutex.Lock()
					mb.lines = append(mb.lines, batch...)
					mb.mutex.Unlock()
				}
				return
			}

			batch = append(batch, v)

			// Drain any additional ready values without blocking
		drain:
			for {
				select {
				case v2, ok2 := <-in:
					if !ok2 {
						if pdebug.Enabled {
							pdebug.Printf("MemoryBuffer input channel closed (read %d lines, %s since starting accept loop)", len(mb.lines)+len(batch), time.Since(start).String())
						}
						mb.mutex.Lock()
						mb.lines = append(mb.lines, batch...)
						mb.mutex.Unlock()
						return
					}
					batch = append(batch, v2)
				default:
					break drain
				}
			}

			// Flush the batch
			mb.mutex.Lock()
			mb.lines = append(mb.lines, batch...)
			mb.mutex.Unlock()
			batch = batch[:0]
		}
	}
}

// AppendLine adds a line to the buffer. This is used by the benchmark tool
// to populate a MemoryBuffer that will be used as a pipeline source.
func (mb *MemoryBuffer) AppendLine(l line.Line) {
	mb.mutex.Lock()
	mb.lines = append(mb.lines, l)
	mb.mutex.Unlock()
}

// LineAt returns the line at the given index, thread-safe.
func (mb *MemoryBuffer) LineAt(n int) (line.Line, error) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return bufferLineAt(mb.lines, n)
}

// linesInRange returns a slice of lines between start and end indices, thread-safe.
func (mb *MemoryBuffer) linesInRange(start, end int) []line.Line {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()
	return mb.lines[start:end]
}

// bufferLineAt is a shared helper that retrieves a line by index from a raw
// line slice, returning an error if the index is out of bounds.
func bufferLineAt(lines []line.Line, n int) (line.Line, error) {
	if s := len(lines); s <= 0 || n >= s {
		return nil, errors.New("empty buffer")
	}

	return lines[n], nil
}

// ContextBuffer holds an expanded view of a filtered buffer, inserting
// context lines from the source around each match (similar to grep -C).
type ContextBuffer struct {
	entries           []line.Line
	maxcols           int
	filtered          Buffer // original filtered buffer, for reference
	matchEntryIndices []int  // maps filtered buffer index -> entries index
}

// NewContextBuffer builds a ContextBuffer by expanding the filtered buffer
// with contextSize lines of surrounding context from the source.
func NewContextBuffer(filtered Buffer, source Buffer, contextSize int) *ContextBuffer {
	cb := &ContextBuffer{
		filtered: filtered,
	}

	filteredSize := filtered.Size()
	if filteredSize == 0 {
		return cb
	}

	sourceSize := source.Size()

	// Step 1: Collect source indices of matched lines
	type matchInfo struct {
		srcIdx      int
		filteredIdx int
		line        line.Line
	}
	matches := make([]matchInfo, 0, filteredSize)
	for i := range filteredSize {
		l, err := filtered.LineAt(i)
		if err != nil {
			continue
		}
		srcIdx := int(l.ID())
		matches = append(matches, matchInfo{srcIdx: srcIdx, filteredIdx: i, line: l})
	}

	if len(matches) == 0 {
		return cb
	}

	// Step 2: Compute context ranges and merge overlapping ones
	type contextRange struct {
		start int // inclusive
		end   int // inclusive
	}
	ranges := make([]contextRange, 0, len(matches))
	for _, m := range matches {
		start := max(m.srcIdx-contextSize, 0)
		end := m.srcIdx + contextSize
		if end >= sourceSize {
			end = sourceSize - 1
		}
		ranges = append(ranges, contextRange{start: start, end: end})
	}

	// Merge overlapping/adjacent ranges
	merged := []contextRange{ranges[0]}
	for i := 1; i < len(ranges); i++ {
		last := &merged[len(merged)-1]
		if ranges[i].start <= last.end+1 {
			if ranges[i].end > last.end {
				last.end = ranges[i].end
			}
		} else {
			merged = append(merged, ranges[i])
		}
	}

	// Step 3: Build a set of matched source indices for quick lookup
	matchedSet := make(map[int]int, len(matches)) // srcIdx -> matches slice index
	for i, m := range matches {
		matchedSet[m.srcIdx] = i
	}

	// Step 4: Build entries
	cb.entries = make([]line.Line, 0, merged[len(merged)-1].end-merged[0].start+1)
	cb.matchEntryIndices = make([]int, filteredSize)
	// Initialize to -1 so unmapped entries are obvious
	for i := range cb.matchEntryIndices {
		cb.matchEntryIndices[i] = -1
	}

	for _, r := range merged {
		for idx := r.start; idx <= r.end; idx++ {
			if mi, ok := matchedSet[idx]; ok {
				// Use the filtered (matched) line, preserving match highlighting
				cb.matchEntryIndices[matches[mi].filteredIdx] = len(cb.entries)
				cb.entries = append(cb.entries, matches[mi].line)
			} else {
				// Context line from source
				srcLine, err := source.LineAt(idx)
				if err != nil {
					continue
				}
				cb.entries = append(cb.entries, &ContextLine{srcLine})
			}
		}
	}

	// Step 5: Compute maxcols
	for _, l := range cb.entries {
		cols := runewidth.StringWidth(l.DisplayString())
		if cols > cb.maxcols {
			cb.maxcols = cols
		}
	}

	return cb
}

// Size returns the number of lines in the context buffer.
func (cb *ContextBuffer) Size() int {
	return len(cb.entries)
}

// LineAt returns the line at index i.
func (cb *ContextBuffer) LineAt(i int) (line.Line, error) {
	if i < 0 || i >= len(cb.entries) {
		return nil, fmt.Errorf("specified index %d is out of range (size=%d)", i, len(cb.entries))
	}
	return cb.entries[i], nil
}

// linesInRange returns a slice of entries between start and end indices.
func (cb *ContextBuffer) linesInRange(start, end int) []line.Line {
	return cb.entries[start:end]
}

// MaxColumn returns the max column size for horizontal scrolling.
func (cb *ContextBuffer) MaxColumn() int {
	return cb.maxcols
}

// MatchEntryIndices returns the mapping from filtered buffer index to
// entries index. Used by ZoomIn to map the cursor position.
func (cb *ContextBuffer) MatchEntryIndices() []int {
	return cb.matchEntryIndices
}

// MemoryBufferSource wraps a completed MemoryBuffer as a pipeline.Source,
// allowing previous filter results to be reused as the input for
// incremental filtering.
type MemoryBufferSource struct {
	buf *MemoryBuffer
}

// NewMemoryBufferSource creates a new MemoryBufferSource from an existing
// MemoryBuffer. The buffer should be fully populated (pipeline completed).
func NewMemoryBufferSource(buf *MemoryBuffer) *MemoryBufferSource {
	return &MemoryBufferSource{buf: buf}
}

// Start iterates through the MemoryBuffer's lines and sends them
// individually to the output channel, implementing pipeline.Source.
// The output channel is closed when all lines have been sent.
func (s *MemoryBufferSource) Start(ctx context.Context, out pipeline.ChanOutput) {
	defer close(out)

	s.buf.mutex.RLock()
	lines := s.buf.lines
	s.buf.mutex.RUnlock()

	for _, l := range lines {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if err := out.Send(ctx, l); err != nil {
			return
		}
	}
}

// Reset is a no-op for MemoryBufferSource since the underlying buffer
// is immutable (from a completed pipeline run).
func (s *MemoryBufferSource) Reset() {}
