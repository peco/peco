package hub

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"time"

	pdebug "github.com/lestrrat-go/pdebug"
)

// Hub acts as the messaging hub between components -- that is,
// it controls how the communication that goes through channels
// are handled.
type Hub struct {
	mutex       sync.Mutex
	queryCh     chan *Payload[string]
	drawCh      chan *Payload[*DrawOptions]
	statusMsgCh chan *Payload[StatusMsg]
	pagingCh    chan *Payload[PagingRequest]
}

// Payload is a wrapper around the actual request value that needs
// to be passed. It contains an optional channel field which can
// be filled to force synchronous communication between the
// sender and receiver
type Payload[T any] struct {
	batch bool
	data  T
	done  chan struct{}
}

// NewPayload creates a new Payload with the given data and batch flag.
func NewPayload[T any](data T, batch bool) *Payload[T] {
	return &Payload[T]{
		data:  data,
		batch: batch,
	}
}

// Batch returns true if this payload is part of a batch operation.
func (p *Payload[T]) Batch() bool {
	return p.batch
}

// Data returns the underlying data.
func (p *Payload[T]) Data() T {
	return p.data
}

// Done marks the request as done. In non-batch mode it's a no-op.
// In batch mode it signals the sender that the receiver has finished
// processing this payload.
func (p *Payload[T]) Done() {
	if p.done == nil {
		return
	}
	p.done <- struct{}{}
}

// New creates a new Hub struct
func New(bufsiz int) *Hub {
	return &Hub{
		queryCh:     make(chan *Payload[string], bufsiz),
		drawCh:      make(chan *Payload[*DrawOptions], bufsiz),
		statusMsgCh: make(chan *Payload[StatusMsg], bufsiz),
		pagingCh:    make(chan *Payload[PagingRequest], bufsiz),
	}
}

type batchPayloadKey struct{}

// batchLockKey is used to detect re-entrant Batch calls so that
// nested calls skip mutex acquisition and avoid deadlock.
type batchLockKey struct{}

// Batch allows you to synchronously send messages during the
// scope of f() being executed. The mutex is acquired automatically
// unless this is a nested Batch call (detected via context).
func (h *Hub) Batch(ctx context.Context, f func(ctx context.Context)) {
	nested, _ := ctx.Value(batchLockKey{}).(bool)

	if pdebug.Enabled {
		g := pdebug.Marker("Batch (nested=%t)", nested)
		defer g.End()
	}

	if !nested {
		h.mutex.Lock()
		defer h.mutex.Unlock()
	}

	// Log and re-panic instead of silently swallowing. The mutex
	// unlock defer (above) still runs during re-panic, so the hub
	// won't deadlock.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "peco: panic in Hub.Batch: %v\n%s", r, debug.Stack())
			panic(r)
		}
	}()

	batchCtx := context.WithValue(ctx, batchPayloadKey{}, true)
	batchCtx = context.WithValue(batchCtx, batchLockKey{}, true)
	f(batchCtx)
}

var doneChPool = sync.Pool{
	New: func() any {
		return make(chan struct{})
	},
}

// waitDone blocks until the receiver signals completion by calling Done.
func (p *Payload[T]) waitDone() {
	// Save the channel reference before blocking. This read is safe because
	// p.done was set by send() on this same goroutine before the payload
	// was sent on the hub channel.
	ch := p.done

	// Block until the receiver calls Done(), which sends on ch.
	<-ch

	// After the receive, the receiver is finished with p.done (it already
	// sent on it), so this goroutine has exclusive access. Clear the field
	// and return the channel to the pool.
	p.done = nil
	doneChPool.Put(ch)
}

// isBatchCtx reports whether the context was created by a Batch call.
func isBatchCtx(ctx context.Context) bool {
	var isBatchMode bool
	v := ctx.Value(batchPayloadKey{})
	if vv, ok := v.(bool); ok {
		isBatchMode = vv
	}
	return isBatchMode
}

// send is the low-level generic utility for sending typed payloads.
// Batch mode is determined from the payload's Batch() flag, which is
// set by each Send* method via isBatchCtx before calling send.
// The context is used for cancellation so sends don't block forever
// during shutdown.
func send[T any](ctx context.Context, ch chan *Payload[T], r *Payload[T]) {
	if pdebug.Enabled {
		g := pdebug.Marker("hub.send (isBatchMode=%t)", r.Batch())
		defer g.End()
	}

	if r.Batch() {
		r.done, _ = doneChPool.Get().(chan struct{})
		if pdebug.Enabled {
			defer pdebug.Printf("request is part of batch operation. waiting")
		}
		defer r.waitDone()
	}

	select {
	case ch <- r:
	case <-ctx.Done():
	}
}

// QueryCh returns the underlying channel for queries
func (h *Hub) QueryCh() chan *Payload[string] {
	return h.queryCh
}

// SendQuery sends the query string to be processed by the Filter
func (h *Hub) SendQuery(ctx context.Context, q string) {
	send(ctx, h.QueryCh(), NewPayload(q, isBatchCtx(ctx)))
}

// DrawCh returns the channel to redraw the terminal display
func (h *Hub) DrawCh() chan *Payload[*DrawOptions] {
	return h.drawCh
}

// SendDrawPrompt sends a request to redraw the prompt only
func (h *Hub) SendDrawPrompt(ctx context.Context) {
	send(ctx, h.DrawCh(), NewPayload(&DrawOptions{Prompt: true}, isBatchCtx(ctx)))
}

// SendDraw sends a request to redraw the terminal display
func (h *Hub) SendDraw(ctx context.Context, options *DrawOptions) {
	if pdebug.Enabled {
		pdebug.Printf("START Hub.SendDraw %v", options)
		defer pdebug.Printf("END Hub.SendDraw %v", options)
	}
	send(ctx, h.DrawCh(), NewPayload(options, isBatchCtx(ctx)))
}

// StatusMsgCh returns the channel to update the status message
func (h *Hub) StatusMsgCh() chan *Payload[StatusMsg] {
	return h.statusMsgCh
}

// SendStatusMsg sends a string to be displayed in the status message.
// If clearDelay is non-zero, the message will be cleared after that duration.
func (h *Hub) SendStatusMsg(ctx context.Context, q string, clearDelay time.Duration) {
	msg := newStatusMsgReq(q, clearDelay)
	send(ctx, h.StatusMsgCh(), NewPayload[StatusMsg](msg, isBatchCtx(ctx)))
}

// StatusMsg is an interface for status message requests.
type StatusMsg interface {
	Message() string
	Delay() time.Duration
}

type statusMsgReq struct {
	msg   string
	delay time.Duration
}

func (r statusMsgReq) Message() string {
	return r.msg
}

func (r statusMsgReq) Delay() time.Duration {
	return r.delay
}

// newStatusMsgReq creates a StatusMsg with the given message text and display duration.
func newStatusMsgReq(s string, d time.Duration) *statusMsgReq {
	return &statusMsgReq{
		msg:   s,
		delay: d,
	}
}

// PagingCh returns the channel to page through the results
func (h *Hub) PagingCh() chan *Payload[PagingRequest] {
	return h.pagingCh
}

// SendPaging sends a request to move the cursor around
func (h *Hub) SendPaging(ctx context.Context, x PagingRequest) {
	send(ctx, h.PagingCh(), NewPayload(x, isBatchCtx(ctx)))
}
