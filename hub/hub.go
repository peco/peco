package hub

import (
	"context"
	"sync"
	"time"

	pdebug "github.com/lestrrat-go/pdebug"
)

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

// Done marks the request as done. If Hub is operating in
// asynchronous mode (default), it's a no op. Otherwise it
// closes the reply channel to finish up the synchronous communication.
func (p *Payload[T]) Done() {
	if p.done == nil {
		return
	}
	p.done <- struct{}{}
}

// New creates a new Hub struct
func New(bufsiz int) *Hub {
	return &Hub{
		isSync:      false,
		queryCh:     make(chan *Payload[string], bufsiz),
		drawCh:      make(chan *Payload[*DrawOptions], bufsiz),
		statusMsgCh: make(chan *Payload[StatusMsg], bufsiz),
		pagingCh:    make(chan *Payload[PagingRequest], bufsiz),
	}
}

type operationNameKey struct{}
type batchPayloadKey struct{}

// Batch allows you to synchronously send messages during the
// scope of f() being executed.
func (h *Hub) Batch(ctx context.Context, f func(ctx context.Context), shouldLock bool) {
	if pdebug.Enabled {
		g := pdebug.Marker("Batch (shouldLock=%t)", shouldLock)
		defer g.End()
	}

	if shouldLock {
		// lock during this operation
		h.mutex.Lock()
		defer h.mutex.Unlock()
	}

	// ignore panics
	defer func() { recover() }()

	f(context.WithValue(ctx, batchPayloadKey{}, true))
}

var doneChPool = sync.Pool{
	New: func() interface{} {
		return make(chan struct{})
	},
}

func (p *Payload[T]) waitDone() {
	// MAKE SURE p.done is valid. XXX needs locking?
	<-p.done

	ch := p.done
	p.done = nil

	defer doneChPool.Put(ch)
}

func isBatchCtx(ctx context.Context) bool {
	var isBatchMode bool
	v := ctx.Value(batchPayloadKey{})
	if vv, ok := v.(bool); ok {
		isBatchMode = vv
	}
	return isBatchMode
}

// send is the low-level generic utility for sending typed payloads.
func send[T any](ctx context.Context, ch chan *Payload[T], r *Payload[T]) {
	isBatchMode := isBatchCtx(ctx)
	if pdebug.Enabled {
		g := pdebug.Marker("hub.send (name=%s, isBatchMode=%t)", ctx.Value(operationNameKey{}), isBatchMode)
		defer g.End()
	}

	if isBatchMode {
		r.done = doneChPool.Get().(chan struct{})
		if pdebug.Enabled {
			defer pdebug.Printf("request is part of batch operation. waiting")
		}
		defer r.waitDone()
	}

	ch <- r
}

// QueryCh returns the underlying channel for queries
func (h *Hub) QueryCh() chan *Payload[string] {
	return h.queryCh
}

// SendQuery sends the query string to be processed by the Filter
func (h *Hub) SendQuery(ctx context.Context, q string) {
	send(context.WithValue(ctx, operationNameKey{}, "send query"), h.QueryCh(), NewPayload(q, isBatchCtx(ctx)))
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
	pdebug.Printf("START Hub.SendDraw %v", options)
	defer pdebug.Printf("END Hub.SendDraw %v", options)
	send(ctx, h.DrawCh(), NewPayload(options, isBatchCtx(ctx)))
}

// SendPurgeDisplayCache sends a request to purge the display cache
func (h *Hub) SendPurgeDisplayCache(ctx context.Context) {
	send(ctx, h.DrawCh(), NewPayload(&DrawOptions{PurgeCache: true}, isBatchCtx(ctx)))
}

// StatusMsgCh returns the channel to update the status message
func (h *Hub) StatusMsgCh() chan *Payload[StatusMsg] {
	return h.statusMsgCh
}

// SendStatusMsg sends a string to be displayed in the status message
func (h *Hub) SendStatusMsg(ctx context.Context, q string) {
	h.SendStatusMsgAndClear(ctx, q, 0)
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

func newStatusMsgReq(s string, d time.Duration) *statusMsgReq {
	return &statusMsgReq{
		msg:   s,
		delay: d,
	}
}

// SendStatusMsgAndClear sends a string to be displayed in the status message,
// as well as a delay until the message should be cleared
func (h *Hub) SendStatusMsgAndClear(ctx context.Context, q string, clearDelay time.Duration) {
	msg := newStatusMsgReq(q, clearDelay)
	send(ctx, h.StatusMsgCh(), NewPayload[StatusMsg](msg, isBatchCtx(ctx)))
}

// PagingCh returns the channel to page through the results
func (h *Hub) PagingCh() chan *Payload[PagingRequest] {
	return h.pagingCh
}

// SendPaging sends a request to move the cursor around
func (h *Hub) SendPaging(ctx context.Context, x PagingRequest) {
	send(ctx, h.PagingCh(), NewPayload(x, isBatchCtx(ctx)))
}
