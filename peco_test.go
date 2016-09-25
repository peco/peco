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
	idgen := newIDGen()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go idgen.Run(ctx)

	lines := []*RawLine{}
	for i := 0; i < 1000000; i++ {
		lines = append(lines, NewRawLine(idgen.next(), fmt.Sprintf("%d", i), false))
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
	if !assert.True(t, util.IsIgnorableError(err), "p.Run() should return error with Ignorable() method, and it should return true") {
		return
	}
}

func TestGHIssue331(t *testing.T) {
	// Note: we should check that the drawing process did not
	// use cached display, but ATM this seemed hard to do,
	// so we just check that the proper fields were populated
	// when peco was instantiated
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, cancel)

	p := newPeco()
	p.Run(ctx)

	if !assert.NotEmpty(t, p.singleKeyJumpPrefixes, "singleKeyJumpPrefixes is not empty") {
		return
	}
	if !assert.NotEmpty(t, p.singleKeyJumpPrefixMap, "singleKeyJumpPrefixMap is not empty") {
		return
	}
}

func TestApplyConfig(t *testing.T) {
	// XXX We should add all the possible configurations that needs to be
	// propagated to Peco from config

	// This is a placeholder test address
	// https://github.com/peco/peco/pull/338#issuecomment-244462220
	var opts CLIOptions

	opts.OptPrompt = "tpmorp>"
	opts.OptQuery = "Hello, World"
	opts.OptBufferSize = 256
	opts.OptInitialIndex = 2
	opts.OptInitialFilter = "Regexp"
	opts.OptLayout = "bottom-up"
	opts.OptSelect1 = true

	p := newPeco()
	if !assert.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed") {
		return
	}

	if !assert.Equal(t, opts.OptQuery, p.initialQuery, "p.initialQuery should be equal to opts.Query") {
		return
	}

	if !assert.Equal(t, opts.OptBufferSize, p.bufferSize, "p.bufferSize should be equal to opts.BufferSize") {
		return
	}

	if !assert.Equal(t, opts.OptEnableNullSep, p.enableSep, "p.enableSep should be equal to opts.OptEnableNullSep") {
		return
	}

	if !assert.Equal(t, opts.OptInitialIndex, p.Location().LineNumber(), "p.Location().LineNumber() should be equal to opts.OptInitialIndex") {
		return
	}

	if !assert.Equal(t, opts.OptInitialFilter, p.filters.Current().String(), "p.initialFilter should be equal to opts.OptInitialFilter") {
		return
	}

	if !assert.Equal(t, opts.OptPrompt, p.prompt, "p.prompt should be equal to opts.OptPrompt") {
		return
	}

	if !assert.Equal(t, opts.OptLayout, p.layoutType, "p.layoutType should be equal to opts.OptLayout") {
		return
	}

	if !assert.Equal(t, opts.OptSelect1, p.selectOneAndExit, "p.selectOneAndExit should be equal to opts.OptSelect1") {
		return
	}
}
