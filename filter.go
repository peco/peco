package peco

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/buffer"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
)

// Filter is responsible for the actual "grep" part of peco
type Filter struct {
	state          *Peco
	prevQuery      string
	prevResults    *MemoryBuffer
	prevFilterName string
	prevMu         sync.Mutex
}

type filterProcessor struct {
	filter  filter.Filter
	query   string
	bufSize int
	onError func(error)
}

func newFilterProcessor(f filter.Filter, q string, bufSize int, onError func(error)) *filterProcessor {
	return &filterProcessor{
		filter:  f,
		query:   q,
		bufSize: bufSize,
		onError: onError,
	}
}

func (fp *filterProcessor) Accept(ctx context.Context, in <-chan line.Line, out pipeline.ChanOutput) {
	acceptAndFilter(ctx, fp.filter, fp.bufSize, fp.onError, in, out)
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

// reportFilterError calls onError with a non-context-cancellation error.
// If the context is already cancelled or onError is nil, it does nothing.
func reportFilterError(ctx context.Context, err error, onError func(error)) {
	if err == nil || ctx.Err() != nil || onError == nil {
		return
	}
	onError(err)
}

// flusher is the single-threaded fallback used when the filter does not
// support parallel execution (e.g. Fuzzy with sortLongest).
func flusher(ctx context.Context, f filter.Filter, incoming chan []line.Line, done chan struct{}, out pipeline.ChanOutput, onError func(error)) {
	if pdebug.Enabled {
		g := pdebug.Marker("flusher goroutine")
		defer g.End()
	}

	defer close(done)
	defer close(out)

	for {
		select {
		case <-ctx.Done():
			return
		case buf, ok := <-incoming:
			if !ok {
				return
			}
			if pdebug.Enabled {
				pdebug.Printf("flusher: %#v", buf)
			}
			if err := f.Apply(ctx, buf, out); err != nil {
				reportFilterError(ctx, err, onError)
			}
			buffer.ReleaseLineListBuf(buf)
		}
	}
}

// parallelFlusher distributes filter work across multiple goroutines
// and merges the results back in sequence order.
func parallelFlusher(ctx context.Context, f filter.Filter, incoming chan orderedChunk, done chan struct{}, out pipeline.ChanOutput, onError func(error)) {
	if pdebug.Enabled {
		g := pdebug.Marker("parallelFlusher goroutine")
		defer g.End()
	}

	defer close(done)
	defer close(out)

	numWorkers := max(runtime.GOMAXPROCS(0), 1)

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
	for range numWorkers {
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
					var err error
					matched, err = collector.ApplyCollect(ctx, chunk.lines)
					if err != nil {
						reportFilterError(ctx, err, onError)
					}
				} else {
					// Fallback: use channel-based Apply for filters that
					// don't implement Collector (e.g. ExternalCmd)
					collectCh := make(chan line.Line, len(chunk.lines))
					go func(chunk orderedChunk) {
						if err := f.Apply(ctx, chunk.lines, pipeline.ChanOutput(collectCh)); err != nil {
							reportFilterError(ctx, err, onError)
						}
						close(collectCh)
					}(chunk)
					matched = make([]line.Line, 0, len(chunk.lines)/2)
					for l := range collectCh {
						matched = append(matched, l)
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
				if err := out.Send(ctx, l); err != nil {
					return
				}
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
func AcceptAndFilter(ctx context.Context, f filter.Filter, configBufSize int, in <-chan line.Line, out pipeline.ChanOutput) {
	acceptAndFilter(ctx, f, configBufSize, nil, in, out)
}

func acceptAndFilter(ctx context.Context, f filter.Filter, configBufSize int, onError func(error), in <-chan line.Line, out pipeline.ChanOutput) {
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
		acceptAndFilterParallel(ctx, f, bufsiz, buf, onError, in, out)
	} else {
		acceptAndFilterSerial(ctx, f, bufsiz, buf, onError, in, out)
	}
}

func acceptAndFilterSerial(ctx context.Context, f filter.Filter, bufsiz int, buf []line.Line, onError func(error), in <-chan line.Line, out pipeline.ChanOutput) {
	flush := make(chan []line.Line)
	flushDone := make(chan struct{})
	go flusher(ctx, f, flush, flushDone, out, onError)
	defer func() { <-flushDone }()
	defer close(flush)

	batchAndFlush(ctx, bufsiz, buf, in, flush, func(b []line.Line) []line.Line { return b })
}

func acceptAndFilterParallel(ctx context.Context, f filter.Filter, bufsiz int, buf []line.Line, onError func(error), in <-chan line.Line, out pipeline.ChanOutput) {
	flush := make(chan orderedChunk)
	flushDone := make(chan struct{})
	go parallelFlusher(ctx, f, flush, flushDone, out, onError)
	defer func() { <-flushDone }()
	defer close(flush)

	seq := 0
	batchAndFlush(ctx, bufsiz, buf, in, flush, func(b []line.Line) orderedChunk {
		chunk := orderedChunk{seq: seq, lines: b}
		seq++
		return chunk
	})
}

// batchAndFlush reads lines from in, batches them into slices of up to bufsiz,
// and sends each batch to flushCh via the wrap function. Batches are flushed
// when full or every 50ms, whichever comes first.
func batchAndFlush[T any](ctx context.Context, bufsiz int, buf []line.Line, in <-chan line.Line, flushCh chan T, wrap func([]line.Line) T) {
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
				flushCh <- wrap(buf)
				buf = buffer.GetLineListBuf()
			}
		case v, ok := <-in:
			if !ok {
				if pdebug.Enabled {
					pdebug.Printf("filter input closed (read %d lines, %s since starting accept loop)", lines+len(buf), time.Since(start).String())
				}
				if len(buf) > 0 {
					flushCh <- wrap(buf)
				}
				return
			}
			if pdebug.Enabled {
				pdebug.Printf("incoming line")
				lines++
			}
			buf = append(buf, v)
			if len(buf) >= bufsiz {
				flushCh <- wrap(buf)
				buf = buffer.GetLineListBuf()
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
// With negative terms, refinement requires:
// 1. Positive portion of prev is a prefix of positive portion of new
// 2. All previous negative terms are still present in new
// 3. New query may have additional positive or negative terms
func isQueryRefinement(prev, cur string) bool {
	prev = strings.TrimSpace(prev)
	cur = strings.TrimSpace(cur)
	if prev == "" || cur == "" {
		return false
	}

	prevPos, prevNeg := filter.SplitQueryTerms(prev)
	newPos, newNeg := filter.SplitQueryTerms(cur)

	// Positive portion: the joined prev positive terms must be a prefix of the joined new positive terms
	prevPosStr := strings.Join(prevPos, " ")
	newPosStr := strings.Join(newPos, " ")
	if prevPosStr != "" && !strings.HasPrefix(newPosStr, prevPosStr) {
		return false
	}

	// All previous negative terms must still be present in new negative terms
	if len(prevNeg) > 0 {
		newNegSet := make(map[string]struct{}, len(newNeg))
		for _, t := range newNeg {
			newNegSet[t] = struct{}{}
		}
		for _, t := range prevNeg {
			if _, ok := newNegSet[t]; !ok {
				return false
			}
		}
	}

	// At least one positive or negative term must exist in both
	if len(prevPos) == 0 && len(prevNeg) == 0 {
		return false
	}

	return true
}

// Work is the actual work horse that does the matching
// in a goroutine of its own. It wraps Matcher.Match().
func (f *Filter) Work(ctx context.Context, q *hub.Payload[string]) {
	defer q.Done()

	query := q.Data()

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

		state.ResetCurrentLineBuffer(ctx)
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
	var srcSize int
	f.prevMu.Lock()
	if f.prevResults != nil &&
		f.prevFilterName == filterName &&
		isQueryRefinement(f.prevQuery, query) {
		if pdebug.Enabled {
			pdebug.Printf("Using incremental source (prev=%q, new=%q, prevSize=%d)", f.prevQuery, query, f.prevResults.Size())
		}
		srcSize = f.prevResults.Size()
		src = NewMemoryBufferSource(f.prevResults)
	}
	f.prevMu.Unlock()

	if src == nil {
		if fs := state.Frozen().Source(); fs != nil {
			src = NewMemoryBufferSource(fs)
			srcSize = fs.Size()
		} else {
			src = state.Source()
			if sizer, ok := src.(interface{ Size() int }); ok {
				srcSize = sizer.Size()
			}
		}
	}
	p.SetSource(src)

	ctx = selectedFilter.NewContext(ctx, query)
	// Report non-cancellation filter errors (e.g. regex compilation failures)
	// to the status bar so the user can see why results are missing.
	onFilterError := func(err error) {
		state.Hub().SendStatusMsg(ctx, err.Error(), 5*time.Second)
	}
	p.Add(newFilterProcessor(selectedFilter, query, state.config.FilterBufSize, onFilterError))

	buf := NewMemoryBuffer(srcSize / 4)
	p.SetDestination(buf)
	state.SetCurrentLineBuffer(ctx, buf)

	go func(ctx context.Context) {
		defer state.Hub().SendDraw(ctx, &hub.DrawOptions{RunningQuery: true})
		if err := p.Run(ctx); err != nil {
			state.Hub().SendStatusMsg(ctx, err.Error(), 0)
		}
	}(ctx)

	go func() {
		if pdebug.Enabled {
			g := pdebug.Marker("Periodic draw request for '%s'", query)
			defer g.End()
		}
		t := time.NewTicker(50 * time.Millisecond)
		defer t.Stop()
		defer state.Hub().SendStatusMsg(ctx, "", 0)
		defer state.Hub().SendDraw(ctx, &hub.DrawOptions{RunningQuery: true})
		for {
			select {
			case <-p.Done():
				return
			case <-t.C:
				state.Hub().SendDraw(ctx, &hub.DrawOptions{RunningQuery: true})
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

	// wg tracks in-flight Work goroutines so Loop doesn't return
	// while they are still running. This ensures clean shutdown
	// without blocking new queries from starting immediately.
	var wg sync.WaitGroup
	defer wg.Wait()

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

			f.state.Hub().SendStatusMsg(ctx, "Running query...", 0)

			wg.Go(func() {
				f.Work(workctx, q)
			})
		}
	}
}
