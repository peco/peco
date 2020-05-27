package peco

import (
	"sync"
	"time"

	"context"

	"github.com/lestrrat-go/pdebug/v2"
	"github.com/peco/peco/buffer"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/pool"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/peco/peco/ui"
)

func newFilterProcessor(f filter.Filter, q string) *filterProcessor {
	return &filterProcessor{
		filter: f,
		query:  q,
	}
}

func (fp *filterProcessor) Accept(ctx context.Context, in chan interface{}, out pipeline.ChanOutput) {
	acceptAndFilter(ctx, fp.filter, in, out)
}

// This flusher is run in a separate goroutine so that the filter can
// run separately from accepting incoming messages
func flusher(ctx context.Context, f filter.Filter, incoming chan []line.Line, done chan struct{}, out pipeline.ChanOutput) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "flusher goroutine")
		defer g.End()
	}

	defer close(done)
	defer func() { _ = out.SendEndMark("end of filter") }()

	for {
		select {
		case <-ctx.Done():
			return
		case buf, ok := <-incoming:
			if !ok {
				return
			}
			pdebug.Printf(ctx, "flusher: %#v", buf)
			_ = f.Apply(ctx, buf, out)
			pool.ReleaseLineListBuf(buf)
		}
	}
}

func acceptAndFilter(ctx context.Context, f filter.Filter, in chan interface{}, out pipeline.ChanOutput) {
	flush := make(chan []line.Line)
	flushDone := make(chan struct{})
	go flusher(ctx, f, flush, flushDone, out)

	buf := pool.GetLineListBuf()
	bufsiz := f.BufSize()
	if bufsiz <= 0 {
		bufsiz = cap(buf)
	}
	defer func() { <-flushDone }() // Wait till the flush goroutine is done
	defer close(flush)             // Kill the flush goroutine

	flushTicker := time.NewTicker(50 * time.Millisecond)
	defer flushTicker.Stop()

	start := time.Now()
	lines := 0
	for {
		select {
		case <-ctx.Done():
			if pdebug.Enabled {
				pdebug.Printf(ctx, "filter received done")
			}
			return
		case <-flushTicker.C:
			if len(buf) > 0 {
				flush <- buf
				buf = pool.GetLineListBuf()
			}
		case v := <-in:
			switch v := v.(type) {
			case error:
				if pipeline.IsEndMark(v) {
					if pdebug.Enabled {
						pdebug.Printf(ctx, "filter received end mark (read %d lines, %s since starting accept loop)", lines+len(buf), time.Since(start).String())
					}
					if len(buf) > 0 {
						flush <- buf
					}
				}
				return
			case line.Line:
				if pdebug.Enabled {
					pdebug.Printf(ctx, "incoming line")
					lines++
				}
				// We buffer the lines so that we can receive more lines to
				// process while we filter what we already have. The buffer
				// size is fairly big, because this really only makes a
				// difference if we have a lot of lines to process.
				buf = append(buf, v.(line.Line))
				if len(buf) >= bufsiz {
					flush <- buf
					buf = pool.GetLineListBuf()
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

// Work is the actual work horse that that does the matching
// in a goroutine of its own. It wraps Matcher.Match().
func (f *Filter) Work(ctx context.Context, q hub.Payload) {
	defer q.Done()

	query, ok := q.Data().(string)
	if !ok {
		return
	}

	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "Filter.Work (query=%#v, batch=%#v)", query, q.Batch())
		defer g.End()
	}

	state := f.state
	if query == "" {
		state.ResetCurrentLineBuffer()
		if !state.config.StickySelection {
			state.Selection().Reset()
		}
		return
	}

	// Create a new pipeline
	p := pipeline.New()
	p.SetSource(state.Source())

	// Wraps the actual filter
	selectedFilter := state.Filters().Current()
	ctx = selectedFilter.NewContext(ctx, query)
	p.Add(newFilterProcessor(selectedFilter, query))

	buf := buffer.NewMemory()
	p.SetDestination(buf)
	state.SetCurrentLineBuffer(buf)

	go func(ctx context.Context) {
		defer state.Hub().SendDraw(ctx, ui.WithRunningQuery(true))
		if err := p.Run(ctx); err != nil {
			state.Hub().SendStatusMsg(ctx, err.Error())
		}
	}(ctx)

	go func() {
		if pdebug.Enabled {
			g := pdebug.Marker(ctx, "Periodic draw request for '%s'", query)
			defer g.End()
		}
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		defer state.Hub().SendStatusMsg(ctx, "")
		defer state.Hub().SendDraw(ctx, ui.WithRunningQuery(true))
		for {
			select {
			case <-p.Done():
				return
			case <-t.C:
				state.Hub().SendDraw(ctx, ui.WithRunningQuery(true))
			}
		}
	}()

	<-p.Done()

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
					pdebug.Printf(ctx, "Canceling previous query")
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
