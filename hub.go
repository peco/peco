package peco

import (
	"sync"
	"time"
)

// Hub acts as the messaging hub between components -- that is,
// it controls how the communication that goes through channels
// are handled.
type Hub struct {
	isSync      bool
	mutex       sync.Locker
	loopCh      chan struct{}
	queryCh     chan HubReq
	drawCh      chan HubReq
	statusMsgCh chan HubReq
	pagingCh    chan HubReq
}

// HubReq is a wrapper around the actual requst value that needs
// to be passed. It contains an optional channel field which can
// be filled to force synchronous communication between the
// sender and receiver
type HubReq struct {
	data    interface{}
	replyCh chan struct{}
}

// DataInterface returns the underlying data as interface{}
func (hr HubReq) DataInterface() interface{} {
	if hr.data == nil {
		return nil
	}
	return hr.data
}

// DataString returns the underlying data as a string. Panics
// if type conversion fails.
func (hr HubReq) DataString() string {
	return hr.data.(string)
}

// Done marks the request as done. If Hub is operating in
// asynchronous mode (default), it's a no op. Otherwise it
// sends a message back the reply channel to finish up the
// synchronous communication
func (hr HubReq) Done() {
	if hr.replyCh == nil {
		return
	}
	hr.replyCh <- struct{}{}
}

// NewHub creates a new Hub struct
func NewHub(bufsiz int) *Hub {
	return &Hub{
		false,
		newMutex(),
		make(chan struct{}),  // loopCh. You never send messages to this. no point in buffering
		make(chan HubReq, bufsiz), // queryCh.
		make(chan HubReq, bufsiz), // drawCh.
		make(chan HubReq, bufsiz), // statusMsgCh
		make(chan HubReq, bufsiz), // pagingCh
	}
}

// Batch allows you to synchronously send messages during the
// scope of f() being executed.
func (h *Hub) Batch(f func()) {
	// lock during this operation
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// temporarily set isSync = true
	o := h.isSync
	h.isSync = true
	defer func() { h.isSync = o }()

	// ignore panics
	defer func() { recover() }()

	f()
}

// low-level utility
func send(ch chan HubReq, r HubReq, needReply bool) {
	if needReply {
		r.replyCh = make(chan struct{})
		defer func() { <-r.replyCh }()
	}

	ch <- r
}

// QueryCh returns the underlying channel for queries
func (h *Hub) QueryCh() chan HubReq {
	return h.queryCh
}

// SendQuery sends the query string to be processed by the Filter
func (h *Hub) SendQuery(q string) {
	send(h.QueryCh(), HubReq{q, nil}, h.isSync)
}

// LoopCh returns the channel to control the main execution loop.
// Nothing should ever be sent through this channel. The only way
// the channel communicates anything to its receivers is when
// it is closed -- which is when peco is done.
func (h *Hub) LoopCh() chan struct{} {
	return h.loopCh
}

// DrawCh returns the channel to redraw the terminal display
func (h *Hub) DrawCh() chan HubReq {
	return h.drawCh
}

// SendDrawPrompt sends a request to redraw the prompt only
func (h *Hub) SendDrawPrompt() {
	req := HubReq{"prompt", nil}
	send(h.DrawCh(), req, h.isSync)
}

// SendDraw sends a request to redraw the terminal display
func (h *Hub) SendDraw() {
	tracer.Printf("Hub.SendDraw: START")
	defer tracer.Printf("Hub.SendDraw: END")
	// to make sure interface is nil, I need to EXPLICITLY set nil
	req := HubReq{nil, nil}
	send(h.DrawCh(), req, h.isSync)
}

// StatusMsgCh returns the channel to update the status message
func (h *Hub) StatusMsgCh() chan HubReq {
	return h.statusMsgCh
}

// SendStatusMsg sends a string to be displayed in the status message
func (h *Hub) SendStatusMsg(q string) {
	h.SendStatusMsgAndClear(q, 0)
}

func (h *Hub) SendStatusMsgAndClear(q string, clearDelay time.Duration) {
	send(h.StatusMsgCh(), HubReq{StatusMsgRequest{q, clearDelay}, nil}, h.isSync)
}

// PagingCh returns the channel to page through the results
func (h *Hub) PagingCh() chan HubReq {
	return h.pagingCh
}

// SendPaging sends a request to move the cursor around
func (h *Hub) SendPaging(x PagingRequest) {
	send(h.PagingCh(), HubReq{x, nil}, h.isSync)
}

// Stop closes the LoopCh so that peco shutsdown
func (h *Hub) Stop() {
	close(h.LoopCh())
}
