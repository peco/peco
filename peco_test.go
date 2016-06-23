package peco

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/internal/util"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

type interceptorArgs []interface{}
type interceptor struct {
	m      sync.Mutex
	events map[string][]interceptorArgs
}

func newInterceptor() *interceptor {
	return &interceptor{
		events: make(map[string][]interceptorArgs),
	}
}

func (i *interceptor) reset() {
	i.m.Lock()
	defer i.m.Unlock()

	i.events = make(map[string][]interceptorArgs)
}

func (i *interceptor) record(name string, args []interface{}) {
	i.m.Lock()
	defer i.m.Unlock()

	events := i.events
	v, ok := events[name]
	if !ok {
		v = []interceptorArgs{}
	}

	events[name] = append(v, interceptorArgs(args))
}

func newPeco() *Peco {
	_, file, _, _ := runtime.Caller(0)
	state := New()
	state.Argv = []string{"peco", file}
	state.screen = NewDummyScreen()
	return state
}

type dummyScreen struct {
	*interceptor
	width  int
	height int
	pollCh chan termbox.Event
}

func NewDummyScreen() *dummyScreen {
	return &dummyScreen{
		interceptor: newInterceptor(),
		width:       80,
		height:      10,
		pollCh:      make(chan termbox.Event),
	}
}

func (d dummyScreen) Init() error {
	return nil
}

func (d dummyScreen) Close() error {
	return nil
}

func (d dummyScreen) Print(args PrintArgs) int {
	return screenPrint(d, args)
}

func (d dummyScreen) SendEvent(e termbox.Event) {
	d.pollCh <- e
}

func (d dummyScreen) SetCell(x, y int, ch rune, fg, bg termbox.Attribute) {
	d.record("SetCell", interceptorArgs{x, y, ch, fg, bg})
}
func (d dummyScreen) Flush() error {
	d.record("Flush", interceptorArgs{})
	return nil
}
func (d dummyScreen) PollEvent() chan termbox.Event {
	return d.pollCh
}
func (d dummyScreen) Size() (int, int) {
	return d.width, d.height
}

func TestIDGen(t *testing.T) {
	lines := []*RawLine{}
	for i := 0; i < 1000000; i++ {
		lines = append(lines, NewRawLine(fmt.Sprintf("%d", i), false))
	}

	sel := NewSelection()
	for _, l := range lines {
		if sel.Has(l) {
			t.Errorf("Collision detected %d", l.ID())
		}
		sel.Add(l)
	}
}

func TestPeco(t *testing.T) {
	p := newPeco()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, cancel)
	if !assert.NoError(t, p.Run(ctx), "p.Run() succeeds") {
		return
	}
}

type testCauser interface {
	Cause() error
}
type testIgnorableError interface {
	Ignorable() bool
}

func TestPecoHelp(t *testing.T) {
	p := newPeco()
	p.Argv = []string{"peco", "-h"}
	p.Stdout = &bytes.Buffer{}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, cancel)

	err := p.Run(ctx)
	if !assert.True(t, util.IsIgnorable(err), "p.Run() should return error with Ignorable() method, and it should return true") {
		return
	}
}
