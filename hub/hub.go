package hub

import (
	"time"

	pdebug "github.com/lestrrat/go-pdebug"
)

func NewPayload(data interface{}) *payload {
	p := &payload{data: data}
	return p
}

// Data returns the underlying data
func (p payload) Data() interface{} {
	return p.data
}

// Done marks the request as done. If Hub is operating in
// asynchronous mode (default), it's a no op. Otherwise it
// closes the reply channel to finish up the synchronous communication
func (p payload) Done() {
	if p.done == nil {
		return
	}
	close(p.done)
}

// NewHub creates a new Hub struct
func New(bufsiz int) *Hub {
	return &Hub{
		isSync:      false,
		queryCh:     make(chan Payload, bufsiz),
		drawCh:      make(chan Payload, bufsiz),
		statusMsgCh: make(chan Payload, bufsiz),
		pagingCh:    make(chan Payload, bufsiz),
	}
}

// Batch allows you to synchronously send messages during the
// scope of f() being executed.
func (h *Hub) Batch(f func(), shouldLock bool) {
	if pdebug.Enabled {
		g := pdebug.Marker("Batch %t", shouldLock)
		defer g.End()
	}
	if shouldLock {
		// lock during this operation
		h.mutex.Lock()
		defer h.mutex.Unlock()
	}

	// temporarily set isSync = true
	o := h.isSync
	h.isSync = true
	defer func() { h.isSync = o }()

	// ignore panics
	defer func() { recover() }()

	f()
}

// low-level utility
func send(ch chan Payload, r *payload, needReply bool) {
	if needReply {
		r.done = make(chan struct{})
		defer func() { <-r.done }()
	}

	ch <- r
}

// QueryCh returns the underlying channel for queries
func (h *Hub) QueryCh() chan Payload {
	return h.queryCh
}

// SendQuery sends the query string to be processed by the Filter
func (h *Hub) SendQuery(q string) {
	send(h.QueryCh(), NewPayload(q), h.isSync)
}

// DrawCh returns the channel to redraw the terminal display
func (h *Hub) DrawCh() chan Payload {
	return h.drawCh
}

// SendDrawPrompt sends a request to redraw the prompt only
func (h *Hub) SendDrawPrompt() {
	send(h.DrawCh(), NewPayload("prompt"), h.isSync)
}

// SendDraw sends a request to redraw the terminal display
func (h *Hub) SendDraw(options interface{}) {
	pdebug.Printf("START Hub.SendDraw %v", options)
	defer pdebug.Printf("END Hub.SendDraw %v", options)
	send(h.DrawCh(), NewPayload(options), h.isSync)
}

// StatusMsgCh returns the channel to update the status message
func (h *Hub) StatusMsgCh() chan Payload {
	return h.statusMsgCh
}

// SendStatusMsg sends a string to be displayed in the status message
func (h *Hub) SendStatusMsg(q string) {
	h.SendStatusMsgAndClear(q, 0)
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
func (h *Hub) SendStatusMsgAndClear(q string, clearDelay time.Duration) {
	msg := newStatusMsgReq(q, clearDelay)
	send(h.StatusMsgCh(), NewPayload(msg), h.isSync)
}

func (h *Hub) SendPurgeDisplayCache() {
	send(h.DrawCh(), NewPayload("purgeCache"), h.isSync)
}

// PagingCh returns the channel to page through the results
func (h *Hub) PagingCh() chan Payload {
	return h.pagingCh
}

// SendPaging sends a request to move the cursor around
func (h *Hub) SendPaging(x interface{}) {
	send(h.PagingCh(), NewPayload(x), h.isSync)
}
