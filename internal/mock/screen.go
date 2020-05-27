package mock

import (
	"context"
	"time"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/ui"
)

type Screen struct {
	*Interceptor
	width  int
	height int
	pollCh chan termbox.Event
}

func NewScreen() *Screen {
	return &Screen{
		Interceptor: NewInterceptor(),
		width:       80,
		height:      10,
		pollCh:      make(chan termbox.Event),
	}
}

func (d Screen) SetCursor(_, _ int) {
}

func (d Screen) Init() error {
	return nil
}

func (d Screen) Close() error {
	return nil
}

func (d Screen) Start() *ui.PrintCtx {
	return ui.NewPrintCtx(d)
}

func (d Screen) SendEvent(e termbox.Event) {
	// XXX FIXME SendEvent should receive a context
	t := time.NewTimer(time.Second)
	defer t.Stop()
	select {
	case <-t.C:
		panic("timed out sending an event")
	case d.pollCh <- e:
	}
}

func (d Screen) SetCell(x, y int, ch rune, fg, bg termbox.Attribute) {
	d.Record("SetCell", []interface{}{x, y, ch, fg, bg})
}
func (d Screen) Flush() error {
	d.Record("Flush", []interface{}{})
	return nil
}
func (d Screen) PollEvent(ctx context.Context) chan termbox.Event {
	return d.pollCh
}
func (d Screen) Size() (int, int) {
	return d.width, d.height
}
func (d Screen) Resume()  {}
func (d Screen) Suspend() {}
