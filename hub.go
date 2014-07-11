package peco

type Hub struct {
	loopCh      chan struct{}
	queryCh     chan string
	drawCh      chan []Match
	statusMsgCh chan string
	pagingCh    chan PagingRequest
}

func NewHub() *Hub {
	return &Hub{
		make(chan struct{}),         // loopCh. You never send messages to this. no point in buffering
		make(chan string, 5),        // queryCh.
		make(chan []Match, 5),       // drawCh.
		make(chan string, 5),        // statusMsgCh
		make(chan PagingRequest, 5), // pagingCh
	}
}

func (h *Hub) LoopCh() chan struct{} {
	return h.loopCh
}

func (h *Hub) QueryCh() chan string {
	return h.queryCh
}

func (h *Hub) DrawCh() chan []Match {
	return h.drawCh
}

func (h *Hub) StatusMsgCh() chan string {
	return h.statusMsgCh
}

func (h *Hub) PagingCh() chan PagingRequest {
	return h.pagingCh
}

func (h *Hub) Stop() {
	close(h.LoopCh())
}
