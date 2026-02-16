package peco

import (
	"runtime"
	"strings"
	"sync"
	"time"

	"context"

	"github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/buffer"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

func newFilterProcessor(f filter.Filter, q string, bufSize int) *filterProcessor {
	return &filterProcessor{
		filter:  f,
		query:   q,
		bufSize: bufSize,
	}
}

func (fp *filterProcessor) Accept(ctx context.Context, in chan interface{}, out pipeline.ChanOutput) {
	acceptAndFilter(ctx, fp.filter, fp.bufSize, in, out)
}

// orderedChunk is a batch of lines tagged with a sequence number
// so that parallel filter results can be merged back in order.
type orderedChunk struct {
	seq   int
	lines []line.Line
}

// orderedResult is a filtered chunk tagged with the original sequence number.
type orderedResult struct {
	seq     int
	matched []line.Line
}

// flusher is the single-threaded fallback used when the filter does not
// support parallel execution (e.g. Fuzzy with sortLongest).
func flusher(ctx context.Context, f filter.Filter, incoming chan []line.Line, done chan struct{}, out pipeline.ChanOutput) {
	if pdebug.Enabled {
		g := pdebug.Marker("flusher goroutine")
		defer g.End()
	}

	defer close(done)
	defer out.SendEndMark(ctx, "end of filter")

	for {
		select {
		case <-ctx.Done():
			return
		case buf, ok := <-incoming:
			if !ok {
				return
			}
			pdebug.Printf("flusher: %#v", buf)
			f.Apply(ctx, buf, out)
			buffer.ReleaseLineListBuf(buf)
		}
	}
}

// parallelFlusher distributes filter work across multiple goroutines
// and merges the results back in sequence order.
func parallelFlusher(ctx context.Context, f filter.Filter, incoming chan orderedChunk, done chan struct{}, out pipeline.ChanOutput) {
	if pdebug.Enabled {
		g := pdebug.Marker("parallelFlusher goroutine")
		defer g.End()
	}

	defer close(done)
	defer out.SendEndMark(ctx, "end of filter")

	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers < 1 {
		numWorkers = 1
	}

	// workCh distributes chunks to workers
	workCh := make(chan orderedChunk, numWorkers*2)
	// resultCh collects filtered results from workers
	resultCh := make(chan orderedResult, numWorkers*2)

	// Check once whether the filter supports direct collection (bypasses
	// per-chunk channel allocation and goroutine spawn).
	collector, canCollect := f.(filter.Collector)

	// Start workers
	var workerWg sync.WaitGroup
	workerWg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer workerWg.Done()
			for chunk := range workCh {
				select {
				case <-ctx.Done():
					buffer.ReleaseLineListBuf(chunk.lines)
					continue
				default:
				}

				var matched []line.Line
				if canCollect {
					// Fast path: collect results directly into a slice
					matched, _ = collector.ApplyCollect(ctx, chunk.lines)
				} else {
					// Fallback: use channel-based Apply for filters that
					// don't implement Collector (e.g. ExternalCmd)
					collectCh := make(chan interface{}, len(chunk.lines))
					go func(chunk orderedChunk) {
						f.Apply(ctx, chunk.lines, pipeline.ChanOutput(collectCh))
						close(collectCh)
					}(chunk)
					matched = make([]line.Line, 0, len(chunk.lines)/2)
					for v := range collectCh {
						if l, ok := v.(line.Line); ok {
							matched = append(matched, l)
						}
					}
				}

				buffer.ReleaseLineListBuf(chunk.lines)

				select {
				case resultCh <- orderedResult{seq: chunk.seq, matched: matched}:
				case <-ctx.Done():
				}
			}
		}()
	}

	// Close resultCh when all workers are done
	go func() {
		workerWg.Wait()
		close(resultCh)
	}()

	// Merger goroutine: reorder results and send to out in sequence order
	mergerDone := make(chan struct{})
	go func() {
		defer close(mergerDone)
		pending := make(map[int]orderedResult)
		nextSeq := 0

		for result := range resultCh {
			pending[result.seq] = result

			// Flush as many in-order results as possible
			for {
				r, ok := pending[nextSeq]
				if !ok {
					break
				}
				delete(pending, nextSeq)
				nextSeq++

				for _, l := range r.matched {
					if err := out.Send(ctx, l); err != nil {
						return
					}
				}
			}
		}

		// Flush any remaining (shouldn't happen if producers are well-behaved)
		for seq := nextSeq; ; seq++ {
			r, ok := pending[seq]
			if !ok {
				break
			}
			for _, l := range r.matched {
				out.Send(ctx, l)
			}
		}
	}()

	// Dispatch incoming chunks to workers
	for chunk := range incoming {
		select {
		case <-ctx.Done():
			buffer.ReleaseLineListBuf(chunk.lines)
		case workCh <- chunk:
		}
	}
	close(workCh)

	// Wait for merger to finish
	<-mergerDone
}

// AcceptAndFilter is the exported entry point for the filter pipeline stage.
// It batches incoming lines and dispatches them to the filter, using parallel
// workers when the filter supports it.
func AcceptAndFilter(ctx context.Context, f filter.Filter, configBufSize int, in chan interface{}, out pipeline.ChanOutput) {
	acceptAndFilter(ctx, f, configBufSize, in, out)
}

func acceptAndFilter(ctx context.Context, f filter.Filter, configBufSize int, in chan interface{}, out pipeline.ChanOutput) {
	useParallel := f.SupportsParallel() && runtime.GOMAXPROCS(0) > 1

	buf := buffer.GetLineListBuf()
	bufsiz := f.BufSize()
	if bufsiz <= 0 {
		if configBufSize > 0 {
			bufsiz = configBufSize
		} else {
			bufsiz = cap(buf)
		}
	}

	if useParallel {
		acceptAndFilterParallel(ctx, f, bufsiz, buf, in, out)
	} else {
		acceptAndFilterSerial(ctx, f, bufsiz, buf, in, out)
	}
}

func acceptAndFilterSerial(ctx context.Context, f filter.Filter, bufsiz int, buf []line.Line, in chan interface{}, out pipeline.ChanOutput) {
	flush := make(chan []line.Line)
	flushDone := make(chan struct{})
	go flusher(ctx, f, flush, flushDone, out)

	defer func() { <-flushDone }()
	defer close(flush)

	flushTicker := time.NewTicker(50 * time.Millisecond)
	defer flushTicker.Stop()

	start := time.Now()
	lines := 0
	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("filter received done")
			}
			return
		case <-flushTicker.C:
			if len(buf) > 0 {
				flush <- buf
				buf = buffer.GetLineListBuf()
			}
		case v := <-in:
			switch v := v.(type) {
			case error:
				if pipeline.IsEndMark(v) {
					if pdebug.Enabled {
						pdebug.Printf("filter received end mark (read %d lines, %s since starting accept loop)", lines+len(buf), time.Since(start).String())
					}
					if len(buf) > 0 {
						flush <- buf
					}
				}
				return
			case line.Line:
				if pdebug.Enabled {
					pdebug.Printf("incoming line")
					lines++
				}
				buf = append(buf, v)
				if len(buf) >= bufsiz {
					flush <- buf
					buf = buffer.GetLineListBuf()
				}
			}
		}
	}
}

func acceptAndFilterParallel(ctx context.Context, f filter.Filter, bufsiz int, buf []line.Line, in chan interface{}, out pipeline.ChanOutput) {
	flush := make(chan orderedChunk)
	flushDone := make(chan struct{})
	go parallelFlusher(ctx, f, flush, flushDone, out)

	defer func() { <-flushDone }()
	defer close(flush)

	flushTicker := time.NewTicker(50 * time.Millisecond)
	defer flushTicker.Stop()

	seq := 0
	start := time.Now()
	lines := 0
	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf("filter received done")
			}
			return
		case <-flushTicker.C:
			if len(buf) > 0 {
				flush <- orderedChunk{seq: seq, lines: buf}
				seq++
				buf = buffer.GetLineListBuf()
			}
		case v := <-in:
			switch v := v.(type) {
			case error:
				if pipeline.IsEndMark(v) {
					if pdebug.Enabled {
						pdebug.Printf("filter received end mark (read %d lines, %s since starting accept loop)", lines+len(buf), time.Since(start).String())
					}
					if len(buf) > 0 {
						flush <- orderedChunk{seq: seq, lines: buf}
					}
				}
				return
			case line.Line:
				if pdebug.Enabled {
					pdebug.Printf("incoming line")
					lines++
				}
				buf = append(buf, v)
				if len(buf) >= bufsiz {
					flush <- orderedChunk{seq: seq, lines: buf}
					seq++
					buf = buffer.GetLineListBuf()
				}
			}
		}
	}
}

func NewFilter(state *Peco) *Filter {
	return &Filter{
		state: state,
	}
}

// isQueryRefinement returns true if newQuery is a refinement of prevQuery,
// meaning the new query can only produce a subset of the previous results.
func isQueryRefinement(prev, new string) bool {
	prev = strings.TrimSpace(prev)
	new = strings.TrimSpace(new)
	if prev == "" || new == "" {
		return false
	}
	return strings.HasPrefix(new, prev)
}

// Work is the actual work horse that does the matching
// in a goroutine of its own. It wraps Matcher.Match().
func (f *Filter) Work(ctx context.Context, q hub.Payload) {
	defer q.Done()

	query, ok := q.Data().(string)
	if !ok {
		return
	}

	if pdebug.Enabled {
		g := pdebug.Marker("Filter.Work (query=%#v, batch=%#v)", query, q.Batch())
		defer g.End()
	}

	state := f.state
	if query == "" {
		f.prevMu.Lock()
		f.prevQuery = ""
		f.prevResults = nil
		f.prevFilterName = ""
		f.prevMu.Unlock()

		state.ResetCurrentLineBuffer()
		if !state.config.StickySelection {
			state.Selection().Reset()
		}
		return
	}

	// Create a new pipeline
	p := pipeline.New()

	// Determine the source: use incremental filtering if possible
	selectedFilter := state.Filters().Current()
	filterName := selectedFilter.String()

	var src pipeline.Source
	f.prevMu.Lock()
	if f.prevResults != nil &&
		f.prevFilterName == filterName &&
		isQueryRefinement(f.prevQuery, query) {
		if pdebug.Enabled {
			pdebug.Printf("Using incremental source (prev=%q, new=%q, prevSize=%d)", f.prevQuery, query, f.prevResults.Size())
		}
		src = NewMemoryBufferSource(f.prevResults)
	}
	f.prevMu.Unlock()

	if src == nil {
		src = state.Source()
	}
	p.SetSource(src)

	ctx = selectedFilter.NewContext(ctx, query)
	p.Add(newFilterProcessor(selectedFilter, query, state.config.FilterBufSize))

	buf := NewMemoryBuffer()
	p.SetDestination(buf)
	state.SetCurrentLineBuffer(buf)

	go func(ctx context.Context) {
		defer state.Hub().SendDraw(ctx, &DrawOptions{RunningQuery: true})
		if err := p.Run(ctx); err != nil {
			state.Hub().SendStatusMsg(ctx, err.Error())
		}
	}(ctx)

	go func() {
		if pdebug.Enabled {
			g := pdebug.Marker("Periodic draw request for '%s'", query)
			defer g.End()
		}
		t := time.NewTicker(50 * time.Millisecond)
		defer t.Stop()
		defer state.Hub().SendStatusMsg(ctx, "")
		defer state.Hub().SendDraw(ctx, &DrawOptions{RunningQuery: true})
		for {
			select {
			case <-p.Done():
				return
			case <-t.C:
				state.Hub().SendDraw(ctx, &DrawOptions{RunningQuery: true})
			}
		}
	}()

	<-p.Done()

	// Save results for incremental filtering only if pipeline completed
	// successfully (context not cancelled)
	if ctx.Err() == nil {
		f.prevMu.Lock()
		f.prevQuery = query
		f.prevResults = buf
		f.prevFilterName = filterName
		f.prevMu.Unlock()
	}

	if !state.config.StickySelection {
		state.Selection().Reset()
	}
}

// Loop keeps watching for incoming queries, and upon receiving
// a query, spawns a goroutine to do the heavy work. It also
// checks for previously running queries, so we can avoid
// running many goroutines doing the grep at the same time
func (f *Filter) Loop(ctx context.Context, cancel func()) error {
	defer cancel()

	// previous holds the function that can cancel the previous
	// query. This is used when multiple queries come in succession
	// and the previous query is discarded anyway
	var mutex sync.Mutex
	var previous func()
	for {
		select {
		case <-ctx.Done():
			return nil
		case q := <-f.state.Hub().QueryCh():
			workctx, workcancel := context.WithCancel(ctx)

			mutex.Lock()
			if previous != nil {
				if pdebug.Enabled {
					pdebug.Printf("Canceling previous query")
				}
				previous()
			}
			previous = workcancel
			mutex.Unlock()

			f.state.Hub().SendStatusMsg(ctx, "Running query...")

			go f.Work(workctx, q)
		}
	}
}
