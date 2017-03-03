package peco

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"runtime"
	"sync"
	"testing"
	"time"

	"context"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/stretchr/testify/assert"
)

type nullHub struct{}

func (h nullHub) Batch(_ func(), _ bool)                          {}
func (h nullHub) DrawCh() chan hub.Payload                        { return nil }
func (h nullHub) PagingCh() chan hub.Payload                      { return nil }
func (h nullHub) QueryCh() chan hub.Payload                       { return nil }
func (h nullHub) SendDraw(_ interface{})                          {}
func (h nullHub) SendDrawPrompt()                                 {}
func (h nullHub) SendPaging(_ interface{})                        {}
func (h nullHub) SendQuery(_ string)                              {}
func (h nullHub) SendStatusMsg(_ string)                          {}
func (h nullHub) SendStatusMsgAndClear(_ string, _ time.Duration) {}
func (h nullHub) StatusMsgCh() chan hub.Payload                   { return nil }

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

func newConfig(s string) (string, error) {
	f, err := ioutil.TempFile("", "peco-test-config-")
	if err != nil {
		return "", err
	}

	io.WriteString(f, s)
	f.Close()
	return f.Name(), nil
}

func newPeco() *Peco {
	_, file, _, _ := runtime.Caller(0)
	state := New()
	state.Argv = []string{"peco", file}
	state.screen = NewDummyScreen()
	state.skipReadConfig = true
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
func (d dummyScreen) PollEvent(ctx context.Context) chan termbox.Event {
	return d.pollCh
}
func (d dummyScreen) Size() (int, int) {
	return d.width, d.height
}
func (d dummyScreen) Resume() {}
func (d dummyScreen) Suspend() {}

func TestIDGen(t *testing.T) {
	idgen := newIDGen()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go idgen.Run(ctx)

	lines := []*line.Raw{}
	for i := 0; i < 1000000; i++ {
		lines = append(lines, line.NewRaw(idgen.Next(), fmt.Sprintf("%d", i), false))
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

func TestConfigFuzzyFilter(t *testing.T) {
	var opts CLIOptions
	p := newPeco()

	// Ensure that it's possible to enable the Fuzzy filter
	opts.OptInitialFilter = "Fuzzy"
	if !assert.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed") {
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
	opts.OptOnCancel = "error"
	opts.OptSelectionPrefix = ">"

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

	if !assert.Equal(t, opts.OptOnCancel, p.onCancel, "p.onCancel should be equal to opts.OptOnCancel") {
		return
	}

	if !assert.Equal(t, opts.OptSelectionPrefix, p.selectionPrefix, "p.selectionPrefix should be equal to opts.OptSelectionPrefix") {
		return
	}
}

// While this issue is labeled for Issue363, it tests against 376 as well.
// The test should have caught the bug for 376, but the premise of the test
// itself was wrong
func TestGHIssue363(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	p := newPeco()
	p.Argv = []string{"--select-1"}
	p.Stdin = bytes.NewBufferString("foo\n")
	var out bytes.Buffer
	p.Stdout = &out

	resultCh := make(chan error)
	go func() {
		defer close(resultCh)
		select {
		case <-ctx.Done():
			return
		case resultCh <- p.Run(ctx):
			return
		}
	}()

	select {
	case <-ctx.Done():
		t.Errorf("timeout reached")
		return
	case err := <-resultCh:
		if !assert.True(t, util.IsCollectResultsError(err), "isCollectResultsError") {
			return
		}
	}

	if !assert.NotEqual(t, "foo\n", out.String(), "output should match") {
		return
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) {
	return f(p)
}

func TestGHIssue367(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p := newPeco()
	p.Argv = []string{}
	src := [][]byte{
		[]byte("foo\n"),
		[]byte("bar\n"),
	}
	ac := time.After(50 * time.Millisecond)
	p.Stdin = readerFunc(func(p []byte) (int, error) {
		if ac != nil {
			<-ac
			ac = nil
		}

		if len(src) == 0 {
			return 0, io.EOF
		}

		l := len(src[0])
		copy(p, src[0])
		src = src[1:]
		return l, nil
	})
	buf := bytes.Buffer{}
	p.Stdout = &buf

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		p.Run(ctx)
	}()

	p.Query().Set("bar")

	select {
	case <-time.After(900 * time.Millisecond):
		p.screen.SendEvent(termbox.Event{Key: termbox.KeyEnter})
	}

	<-waitCh

	p.PrintResults()

	curbuf := p.CurrentLineBuffer()

	if !assert.Equal(t, curbuf.Size(), 1, "There should be one element in buffer") {
		return
	}

	for i := 0; i < curbuf.Size(); i++ {
		_, err := curbuf.LineAt(i)
		if !assert.NoError(t, err, "LineAt(%d) should succeed", i) {
			return
		}
	}

	if !assert.Equal(t, "bar\n", buf.String(), "output should match") {
		return
	}
}
