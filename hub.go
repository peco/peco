package peco

import "sync"

type Hub struct {
	isSync      bool
	mutex       *sync.Mutex
	loopCh      chan struct{}
	queryCh     chan HubReq
	drawCh      chan HubReq
	statusMsgCh chan HubReq
	pagingCh    chan HubReq
}

type HubReq struct {
	data    interface{}
	replyCh chan struct{}
}

func (hr HubReq) DataInterface() interface{} {
	return hr.data
}

func (hr HubReq) DataString() string {
	return hr.data.(string)
}

func (hr HubReq) Done() {
	if hr.replyCh != nil {
		hr.replyCh <- struct{}{}
	}
}

func NewHub() *Hub {
	return &Hub{
		false,
		&sync.Mutex{},
		make(chan struct{}),  // loopCh. You never send messages to this. no point in buffering
		make(chan HubReq, 5), // queryCh.
		make(chan HubReq, 5), // drawCh.
		make(chan HubReq, 5), // statusMsgCh
		make(chan HubReq, 5), // pagingCh
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

func (h *Hub) QueryCh() chan HubReq {
	return h.queryCh
}

func (h *Hub) SendQuery(q string) {
	send(h.QueryCh(), HubReq{q, nil}, h.isSync)
}

func (h *Hub) LoopCh() chan struct{} {
	return h.loopCh
}

func (h *Hub) DrawCh() chan HubReq {
	return h.drawCh
}

func (h *Hub) SendDraw(matches []Match) {
	send(h.DrawCh(), HubReq{matches, nil}, h.isSync)
}

func (h *Hub) StatusMsgCh() chan HubReq {
	return h.statusMsgCh
}

func (h *Hub) SendStatusMsg(q string) {
	send(h.StatusMsgCh(), HubReq{q, nil}, h.isSync)
}

func (h *Hub) PagingCh() chan HubReq {
	return h.pagingCh
}

func (h *Hub) SendPaging(x PagingRequest) {
	send(h.PagingCh(), HubReq{x, nil}, h.isSync)
}

func (h *Hub) Stop() {
	close(h.LoopCh())
}
