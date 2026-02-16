package peco

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nullHub struct{}

func (h nullHub) Batch(_ context.Context, _ func(context.Context), _ bool)           {}
func (h nullHub) DrawCh() chan hub.Payload                                           { return nil }
func (h nullHub) PagingCh() chan hub.Payload                                         { return nil }
func (h nullHub) QueryCh() chan hub.Payload                                          { return nil }
func (h nullHub) SendDraw(_ context.Context, _ interface{})                          {}
func (h nullHub) SendDrawPrompt(context.Context)                                     {}
func (h nullHub) SendPaging(_ context.Context, _ interface{})                        {}
func (h nullHub) SendQuery(_ context.Context, _ string)                              {}
func (h nullHub) SendStatusMsg(_ context.Context, _ string)                          {}
func (h nullHub) SendStatusMsgAndClear(_ context.Context, _ string, _ time.Duration) {}
func (h nullHub) StatusMsgCh() chan hub.Payload                                      { return nil }

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
	f, err := os.CreateTemp("", "peco-test-config-")
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

// keyseqToTcellKey maps peco keyseq navigation/function key constants back
// to tcell key constants, for injecting events into SimulationScreen.
var keyseqToTcellKey = map[keyseq.KeyType]tcell.Key{
	keyseq.KeyArrowUp:    tcell.KeyUp,
	keyseq.KeyArrowDown:  tcell.KeyDown,
	keyseq.KeyArrowLeft:  tcell.KeyLeft,
	keyseq.KeyArrowRight: tcell.KeyRight,
	keyseq.KeyInsert:     tcell.KeyInsert,
	keyseq.KeyDelete:     tcell.KeyDelete,
	keyseq.KeyHome:       tcell.KeyHome,
	keyseq.KeyEnd:        tcell.KeyEnd,
	keyseq.KeyPgup:       tcell.KeyPgUp,
	keyseq.KeyPgdn:       tcell.KeyPgDn,
	keyseq.KeyF1:         tcell.KeyF1,
	keyseq.KeyF2:         tcell.KeyF2,
	keyseq.KeyF3:         tcell.KeyF3,
	keyseq.KeyF4:         tcell.KeyF4,
	keyseq.KeyF5:         tcell.KeyF5,
	keyseq.KeyF6:         tcell.KeyF6,
	keyseq.KeyF7:         tcell.KeyF7,
	keyseq.KeyF8:         tcell.KeyF8,
	keyseq.KeyF9:         tcell.KeyF9,
	keyseq.KeyF10:        tcell.KeyF10,
	keyseq.KeyF11:        tcell.KeyF11,
	keyseq.KeyF12:        tcell.KeyF12,
}

// SimScreen wraps tcell.SimulationScreen to implement peco's Screen interface.
// It also embeds interceptor for backward-compatible test assertions.
type SimScreen struct {
	*interceptor
	mu     sync.Mutex
	closed bool
	screen tcell.SimulationScreen
}

// NewDummyScreen creates a SimScreen backed by tcell.SimulationScreen.
func NewDummyScreen() *SimScreen {
	ss := tcell.NewSimulationScreen("")
	ss.Init()
	ss.SetSize(80, 10)
	return &SimScreen{
		interceptor: newInterceptor(),
		screen:      ss,
	}
}

func (s *SimScreen) Init(cfg *Config) error {
	return nil
}

func (s *SimScreen) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.screen.Fini()
	return nil
}

func (s *SimScreen) SetCursor(x, y int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.screen.ShowCursor(x, y)
}

func (s *SimScreen) Print(args PrintArgs) int {
	return screenPrint(s, args)
}

func (s *SimScreen) SendEvent(e Event) {
	var mod tcell.ModMask
	if e.Mod == keyseq.ModAlt {
		mod = tcell.ModAlt
	}

	// Regular character
	if e.Key == 0 && e.Ch != 0 {
		s.screen.InjectKey(tcell.KeyRune, e.Ch, mod)
		return
	}

	// Space: reverse of tcellEventToEvent's special case
	if e.Key == keyseq.KeySpace {
		s.screen.InjectKey(tcell.KeyRune, ' ', mod)
		return
	}

	// Navigation/function keys via reverse lookup table
	if tcellKey, ok := keyseqToTcellKey[e.Key]; ok {
		s.screen.InjectKey(tcellKey, 0, mod)
		return
	}

	// Ctrl keys (0x00-0x1F) and DEL (0x7F): direct cast
	if e.Key <= 0x1F || e.Key == 0x7F {
		s.screen.InjectKey(tcell.Key(e.Key), 0, mod)
		return
	}
}

func (s *SimScreen) SetCell(x, y int, ch rune, fg, bg Attribute) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.record("SetCell", interceptorArgs{x, y, ch, fg, bg})
	style := attributeToTcellStyle(fg, bg)
	s.screen.SetContent(x, y, ch, nil, style)
}

func (s *SimScreen) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.record("Flush", interceptorArgs{})
	s.screen.Show()
	return nil
}

func (s *SimScreen) PollEvent(ctx context.Context, cfg *Config) chan Event {
	evCh := make(chan Event)
	go func() {
		defer func() { recover() }()
		defer close(evCh)

		for {
			ev := s.screen.PollEvent()
			if ev == nil {
				return
			}
			pecoEv := tcellEventToEvent(ev)
			select {
			case <-ctx.Done():
				return
			case evCh <- pecoEv:
			}
		}
	}()
	return evCh
}

func (s *SimScreen) Size() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, 0
	}
	return s.screen.Size()
}

func (s *SimScreen) Resume(_ context.Context) {}
func (s *SimScreen) Suspend() {}

// Sync records a "Sync" event via the interceptor. This satisfies the
// optional syncer interface used by BasicLayout.DrawScreen when
// ForceSync is requested.
func (s *SimScreen) Sync() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.record("Sync", interceptorArgs{})
	s.screen.Sync()
}

func TestIDGen(t *testing.T) {
	idgen := newIDGen()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go idgen.Run(ctx)

	lines := []*line.Raw{}
	for i := 0; i < 1000000; i++ {
		lines = append(lines, line.NewRaw(idgen.Next(), fmt.Sprintf("%d", i), false, false))
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
	opts.OptExitZero = true
	opts.OptSelectAll = true
	opts.OptOnCancel = "error"
	opts.OptSelectionPrefix = ">"
	opts.OptPrintQuery = true

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

	if !assert.Equal(t, opts.OptExitZero, p.exitZeroAndExit, "p.exitZeroAndExit should be equal to opts.OptExitZero") {
		return
	}

	if !assert.Equal(t, opts.OptSelectAll, p.selectAllAndExit, "p.selectAllAndExit should be equal to opts.OptSelectAll") {
		return
	}

	if !assert.Equal(t, OnCancelBehavior(opts.OptOnCancel), p.onCancel, "p.onCancel should be equal to opts.OptOnCancel") {
		return
	}

	if !assert.Equal(t, opts.OptSelectionPrefix, p.selectionPrefix, "p.selectionPrefix should be equal to opts.OptSelectionPrefix") {
		return
	}
	if !assert.Equal(t, opts.OptPrintQuery, p.printQuery, "p.printQuery should be equal to opts.OptPrintQuery") {
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
		p.PrintResults()
	}

	if !assert.Equal(t, "foo\n", out.String(), "output should match") {
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
		p = p[:l]
		src = src[1:]
		if pdebug.Enabled {
			pdebug.Printf("reader func returning %#v", string(p))
		}
		return l, nil
	})
	buf := bytes.Buffer{}
	p.Stdout = &buf

	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		p.Run(ctx)
	}()

	select {
	case <-time.After(100 * time.Millisecond):
		p.screen.SendEvent(Event{Type: EventKey, Ch: 'b'})
	case <-time.After(200 * time.Millisecond):
		p.screen.SendEvent(Event{Type: EventKey, Ch: 'a'})
	case <-time.After(300 * time.Millisecond):
		p.screen.SendEvent(Event{Type: EventKey, Ch: 'r'})
	case <-time.After(900 * time.Millisecond):
		p.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyEnter})
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

func TestExitZero(t *testing.T) {
	t.Run("Empty input exits with status 1", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--exit-0"}
		p.Stdin = bytes.NewBufferString("")
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
			if !assert.True(t, util.IsIgnorableError(err), "error should be ignorable") {
				return
			}
			st, ok := util.GetExitStatus(err)
			if !assert.True(t, ok, "error should have exit status") {
				return
			}
			if !assert.Equal(t, 1, st, "exit status should be 1") {
				return
			}
		}

		if !assert.Empty(t, out.String(), "output should be empty") {
			return
		}
	})

	t.Run("Non-empty input does not auto-exit", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--exit-0"}
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

		// Wait for peco to be ready, then cancel after a short delay
		// If --exit-0 incorrectly triggered, we'd get an ignorable error
		<-p.Ready()
		time.AfterFunc(500*time.Millisecond, cancel)

		select {
		case <-ctx.Done():
			// Expected: peco stayed running until we cancelled
		case err := <-resultCh:
			// If we got a result, it should NOT be an ignorable error with exit status 1
			if util.IsIgnorableError(err) {
				st, ok := util.GetExitStatus(err)
				if ok && st == 1 {
					t.Errorf("--exit-0 should not trigger when input is non-empty")
				}
			}
		}
	})
}

// runPecoSelectAll is a helper that runs peco in a goroutine and waits for
// it to complete. It uses a simple buffered channel to avoid goroutine
// scheduling races between the result send and the context cancellation.
func runPecoSelectAll(t *testing.T, p *Peco, ctx context.Context) {
	t.Helper()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- p.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		t.Fatal("timeout reached")
	case err := <-resultCh:
		require.True(t, util.IsCollectResultsError(err), "isCollectResultsError")
		p.PrintResults()
	}
}

func TestSelectAll(t *testing.T) {
	t.Run("Multiple lines outputs all lines", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--select-all"}
		p.Stdin = bytes.NewBufferString("foo\nbar\nbaz\n")
		var out bytes.Buffer
		p.Stdout = &out

		runPecoSelectAll(t, p, ctx)

		require.Equal(t, "foo\nbar\nbaz\n", out.String(), "output should match")
	})

	t.Run("Single line outputs that line", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--select-all"}
		p.Stdin = bytes.NewBufferString("only\n")
		var out bytes.Buffer
		p.Stdout = &out

		runPecoSelectAll(t, p, ctx)

		require.Equal(t, "only\n", out.String(), "output should match")
	})

	t.Run("Empty input outputs nothing", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--select-all"}
		p.Stdin = bytes.NewBufferString("")
		var out bytes.Buffer
		p.Stdout = &out

		runPecoSelectAll(t, p, ctx)

		require.Empty(t, out.String(), "output should be empty")
	})

	t.Run("With --print-query outputs query then all lines", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--select-all", "--print-query", "--query", "test"}
		p.Stdin = bytes.NewBufferString("foo\nbar\n")
		var out bytes.Buffer
		p.Stdout = &out

		runPecoSelectAll(t, p, ctx)

		require.Equal(t, "test\n", out.String(), "output should have query and no matching lines")
	})

	t.Run("With query filters then selects all matches", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--select-all", "--query", "foo"}
		p.Stdin = bytes.NewBufferString("foo\nbar\nfoobar\n")
		var out bytes.Buffer
		p.Stdout = &out

		runPecoSelectAll(t, p, ctx)

		require.Equal(t, "foo\nfoobar\n", out.String(), "output should contain only matching lines")
	})
}

func TestPrintQuery(t *testing.T) {
	t.Run("Match and print query", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--print-query", "--query", "oo", "--select-1"}
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
			p.PrintResults()
		}

		if !assert.Equal(t, "oo\nfoo\n", out.String(), "output should match") {
			return
		}
	})
	t.Run("No match and print query", func(t *testing.T) { //nolint:dupl
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		p := newPeco()
		p.Argv = []string{"--print-query", "--query", "oo"}
		p.Stdin = bytes.NewBufferString("bar\n")
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

		<-p.Ready()

		time.AfterFunc(100*time.Millisecond, func() {
			p.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyEnter})
		})

		select {
		case <-ctx.Done():
			t.Errorf("timeout reached")
			return
		case err := <-resultCh:
			if !assert.True(t, util.IsCollectResultsError(err), "isCollectResultsError") {
				return
			}
			p.PrintResults()
		}

		if !assert.Equal(t, "oo\n", out.String(), "output should match") {
			return
		}
	})
}

func TestMemoryBufferMarkComplete(t *testing.T) {
	t.Run("signals done channel", func(t *testing.T) {
		mb := NewMemoryBuffer(0)
		mb.MarkComplete()

		select {
		case <-mb.Done():
			// expected
		default:
			t.Fatal("Done channel should be closed after MarkComplete")
		}
	})

	t.Run("idempotent - multiple calls do not panic", func(t *testing.T) {
		mb := NewMemoryBuffer(0)
		mb.MarkComplete()
		mb.MarkComplete() // must not panic
		mb.MarkComplete() // must not panic

		select {
		case <-mb.Done():
			// expected
		default:
			t.Fatal("Done channel should be closed after MarkComplete")
		}
	})

	t.Run("works after Reset", func(t *testing.T) {
		mb := NewMemoryBuffer(0)
		mb.MarkComplete()

		mb.Reset()

		// After reset, done should be a new open channel
		select {
		case <-mb.Done():
			t.Fatal("Done channel should not be closed after Reset")
		default:
			// expected
		}

		// MarkComplete should work again on the new channel
		mb.MarkComplete()

		select {
		case <-mb.Done():
			// expected
		default:
			t.Fatal("Done channel should be closed after second MarkComplete")
		}
	})
}

// TestQueryExecTimerStoppedOnCancel verifies that the queryExecTimer
// is stopped when the context is cancelled, preventing the timer
// callback from firing after program teardown (issue 1.4 / 7.2).
func TestQueryExecTimerStoppedOnCancel(t *testing.T) {
	p := newPeco()
	// Set a long query exec delay so the timer is still pending
	// when we cancel.
	p.queryExecDelay = 5 * time.Second
	p.Stdin = bytes.NewBufferString("foo\nbar\nbaz\n")
	var out bytes.Buffer
	p.Stdout = &out

	ctx, cancel := context.WithCancel(context.Background())

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- p.Run(ctx)
	}()

	// Wait for peco to be ready
	<-p.Ready()

	// Type a character to trigger ExecQuery which will create the timer
	p.screen.SendEvent(Event{Type: EventKey, Ch: 'f'})

	// Give the input loop time to process the keystroke and create the timer
	time.Sleep(100 * time.Millisecond)

	// Verify the timer was created
	p.queryExecMutex.Lock()
	timerCreated := p.queryExecTimer != nil
	p.queryExecMutex.Unlock()
	require.True(t, timerCreated, "queryExecTimer should have been created")

	// Cancel the context (simulating program exit)
	cancel()

	// Wait for Run to return
	<-waitCh

	// After Run returns, the timer should have been stopped.
	p.queryExecMutex.Lock()
	timerAfterCancel := p.queryExecTimer
	p.queryExecMutex.Unlock()
	require.Nil(t, timerAfterCancel, "queryExecTimer should be nil after cancellation")
}
