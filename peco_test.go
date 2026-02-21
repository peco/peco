package peco

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/config"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/keyseq"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/selection"
	"github.com/stretchr/testify/require"
)

type nullHub struct{}

func (h nullHub) Batch(_ context.Context, _ func(context.Context))           {}
func (h nullHub) DrawCh() chan *hub.Payload[*hub.DrawOptions]                { return nil }
func (h nullHub) PagingCh() chan *hub.Payload[hub.PagingRequest]             { return nil }
func (h nullHub) QueryCh() chan *hub.Payload[string]                         { return nil }
func (h nullHub) SendDraw(_ context.Context, _ *hub.DrawOptions)             {}
func (h nullHub) SendDrawPrompt(context.Context)                             {}
func (h nullHub) SendPaging(_ context.Context, _ hub.PagingRequest)          {}
func (h nullHub) SendQuery(_ context.Context, _ string)                      {}
func (h nullHub) SendStatusMsg(_ context.Context, _ string, _ time.Duration) {}
func (h nullHub) StatusMsgCh() chan *hub.Payload[hub.StatusMsg]              { return nil }

// Compile-time interface compliance checks.
var (
	_ MessageHub = (*hub.Hub)(nil)
	_ MessageHub = nullHub{}
)

type interceptorArgs []any
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

func (i *interceptor) record(name string, args []any) {
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
	state.configReader = nopConfigReader
	return state
}

func setupPecoTest(t *testing.T) (*Peco, context.Context) {
	t.Helper()
	state := newPeco()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go state.Run(ctx)
	<-state.Ready()
	return state, ctx
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

func (s *SimScreen) Init(_ *config.Config) error {
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

// keyseqToTcellButton maps peco mouse key constants to tcell button masks.
var keyseqToTcellButton = map[keyseq.KeyType]tcell.ButtonMask{
	keyseq.MouseLeft:   tcell.Button1,
	keyseq.MouseMiddle: tcell.Button2,
	keyseq.MouseRight:  tcell.Button3,
}

func (s *SimScreen) SendEvent(e Event) {
	var mod tcell.ModMask
	if e.Mod == keyseq.ModAlt {
		mod = tcell.ModAlt
	}

	// Mouse events
	if btn, ok := keyseqToTcellButton[e.Key]; ok {
		s.screen.InjectMouse(0, 0, btn, mod)
		return
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

func (s *SimScreen) SetCell(x, y int, ch rune, fg, bg config.Attribute) {
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

func (s *SimScreen) PollEvent(ctx context.Context, _ *config.Config) chan Event {
	evCh := make(chan Event)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "SimScreen: panic in PollEvent goroutine: %v\n%s", r, debug.Stack())
			}
		}()
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

func (s *SimScreen) Resume(_ context.Context) error { return nil }
func (s *SimScreen) Suspend()                       {}

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
	ctx := t.Context()
	go idgen.Run(ctx)

	lines := make([]*line.Raw, 0, 1000000)
	for i := range 1000000 {
		lines = append(lines, line.NewRaw(idgen.Next(), fmt.Sprintf("%d", i), false, false))
	}

	sel := selection.New()
	for _, l := range lines {
		require.False(t, sel.Has(l), "Collision detected %d", l.ID())
		sel.Add(l)
	}
}

func TestPeco(t *testing.T) {
	p := newPeco()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, cancel)
	require.NoError(t, p.Run(ctx), "p.Run() succeeds")
}

func TestPecoHelp(t *testing.T) {
	p := newPeco()
	p.Argv = []string{"peco", "-h"}
	p.Stdout = &bytes.Buffer{}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Second, cancel)

	err := p.Run(ctx)
	require.True(t, util.IsIgnorableError(err), "p.Run() should return error with Ignorable() method, and it should return true")
}

func TestOnCancelInvalidCLIOption(t *testing.T) {
	p := newPeco()
	var opts CLIOptions
	opts.OptOnCancel = "bogus"
	err := p.ApplyConfig(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bogus")
}

func TestGHIssue331(t *testing.T) {
	// Verify fields are populated when Run() initializes config.
	state, _ := setupPecoTest(t)
	require.NotEmpty(t, state.singleKeyJump.prefixes, "singleKeyJump.prefixes should be populated")
	require.NotEmpty(t, state.singleKeyJump.prefixMap, "singleKeyJump.prefixMap should be populated")

	// Verify ToggleSingleKeyJumpMode on a separate non-running instance
	// to avoid racing with the View loop's DrawScreen reads.
	p := New()
	p.hub = nullHub{}
	require.False(t, p.SingleKeyJump().Mode(), "SingleKeyJump().Mode() should start as false")
	p.ToggleSingleKeyJumpMode(context.Background())
	require.True(t, p.SingleKeyJump().Mode(), "SingleKeyJump().Mode() should be true after toggle")
	p.ToggleSingleKeyJumpMode(context.Background())
	require.False(t, p.SingleKeyJump().Mode(), "SingleKeyJump().Mode() should be false after second toggle")
}

func TestConfigFuzzyFilter(t *testing.T) {
	var opts CLIOptions
	p := newPeco()

	// Ensure that it's possible to enable the Fuzzy filter
	opts.OptInitialFilter = "Fuzzy"
	require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")
}

func TestApplyConfig(t *testing.T) {
	// This is a placeholder test address
	// https://github.com/peco/peco/pull/338#issuecomment-244462220

	t.Run("CLI options", func(t *testing.T) {
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
		opts.OptExec = "cat"
		opts.OptColor = config.ColorModeAuto
		opts.OptHeight = "20"

		p := newPeco()
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.Equal(t, opts.OptQuery, p.initialQuery, "p.initialQuery should be equal to opts.OptQuery")
		require.Equal(t, opts.OptBufferSize, p.bufferSize, "p.bufferSize should be equal to opts.OptBufferSize")
		require.Equal(t, opts.OptEnableNullSep, p.enableSep, "p.enableSep should be equal to opts.OptEnableNullSep")
		require.Equal(t, opts.OptInitialIndex, p.Location().LineNumber(), "p.Location().LineNumber() should be equal to opts.OptInitialIndex")
		require.Equal(t, opts.OptInitialFilter, p.filters.Current().String(), "p.initialFilter should be equal to opts.OptInitialFilter")
		require.Equal(t, opts.OptPrompt, p.prompt, "p.prompt should be equal to opts.OptPrompt")
		require.Equal(t, opts.OptLayout, p.layoutType, "p.layoutType should be equal to opts.OptLayout")
		require.Equal(t, opts.OptSelect1, p.selectOneAndExit, "p.selectOneAndExit should be equal to opts.OptSelect1")
		require.Equal(t, opts.OptExitZero, p.exitZeroAndExit, "p.exitZeroAndExit should be equal to opts.OptExitZero")
		require.Equal(t, opts.OptSelectAll, p.selectAllAndExit, "p.selectAllAndExit should be equal to opts.OptSelectAll")
		require.Equal(t, config.OnCancelBehavior(opts.OptOnCancel), p.onCancel, "p.onCancel should be equal to opts.OptOnCancel")
		require.Equal(t, opts.OptSelectionPrefix, p.selectionPrefix, "p.selectionPrefix should be equal to opts.OptSelectionPrefix")
		require.Equal(t, opts.OptPrintQuery, p.printQuery, "p.printQuery should be equal to opts.OptPrintQuery")
		require.Equal(t, opts.OptExec, p.execOnFinish, "p.execOnFinish should be equal to opts.OptExec")
		require.True(t, p.enableANSI, "p.enableANSI should be true when opts.OptColor is 'auto'")
		require.NotNil(t, p.heightSpec, "p.heightSpec should be set when opts.OptHeight is provided")
		require.Equal(t, 20, p.heightSpec.Value, "p.heightSpec.Value should be 20")
		require.False(t, p.heightSpec.IsPercent, "p.heightSpec.IsPercent should be false for absolute value")
	})

	t.Run("CLI options with percentage height", func(t *testing.T) {
		var opts CLIOptions
		opts.OptHeight = "50%"

		p := newPeco()
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.NotNil(t, p.heightSpec, "p.heightSpec should be set")
		require.Equal(t, 50, p.heightSpec.Value, "p.heightSpec.Value should be 50")
		require.True(t, p.heightSpec.IsPercent, "p.heightSpec.IsPercent should be true for percentage")
	})

	t.Run("Config-level fields", func(t *testing.T) {
		p := newPeco()
		p.config.MaxScanBufferSize = 512
		p.config.FuzzyLongestSort = true

		var opts CLIOptions
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.Equal(t, 512, p.maxScanBufferSize, "p.maxScanBufferSize should be equal to config.MaxScanBufferSize")
		require.True(t, p.fuzzyLongestSort, "p.fuzzyLongestSort should be true when config.FuzzyLongestSort is true")
		require.True(t, p.enableANSI, "p.enableANSI should be true by default")
	})

	t.Run("MaxScanBufferSize defaults to 256", func(t *testing.T) {
		p := newPeco()

		var opts CLIOptions
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.Equal(t, 256, p.maxScanBufferSize, "p.maxScanBufferSize should default to 256")
	})

	t.Run("Config height used when CLI option absent", func(t *testing.T) {
		p := newPeco()
		p.config.Height = "30%"

		var opts CLIOptions
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.NotNil(t, p.heightSpec, "p.heightSpec should be set from config")
		require.Equal(t, 30, p.heightSpec.Value, "p.heightSpec.Value should be 30")
		require.True(t, p.heightSpec.IsPercent, "p.heightSpec.IsPercent should be true")
	})

	t.Run("CLI height overrides config height", func(t *testing.T) {
		p := newPeco()
		p.config.Height = "30%"

		var opts CLIOptions
		opts.OptHeight = "10"
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.NotNil(t, p.heightSpec, "p.heightSpec should be set")
		require.Equal(t, 10, p.heightSpec.Value, "p.heightSpec.Value should come from CLI option")
		require.False(t, p.heightSpec.IsPercent, "p.heightSpec.IsPercent should be false for absolute CLI value")
	})

	t.Run("Config OnCancel used when CLI option absent", func(t *testing.T) {
		p := newPeco()
		p.config.OnCancel = config.OnCancelError

		var opts CLIOptions
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.Equal(t, config.OnCancelError, p.onCancel, "p.onCancel should come from config when CLI option is absent")
	})

	t.Run("Config SelectionPrefix used when CLI option absent", func(t *testing.T) {
		p := newPeco()
		p.config.SelectionPrefix = "*"

		var opts CLIOptions
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.Equal(t, "*", p.selectionPrefix, "p.selectionPrefix should come from config when CLI option is absent")
	})

	t.Run("Config Prompt used when CLI option absent", func(t *testing.T) {
		p := newPeco()
		p.config.Prompt = "FIND>"

		var opts CLIOptions
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.Equal(t, "FIND>", p.prompt, "p.prompt should come from config when CLI option is absent")
	})

	t.Run("Config InitialFilter used when CLI option absent", func(t *testing.T) {
		p := newPeco()
		p.config.InitialFilter = "SmartCase"

		var opts CLIOptions
		require.NoError(t, p.ApplyConfig(opts), "p.ApplyConfig should succeed")

		require.Equal(t, "SmartCase", p.filters.Current().String(), "p.filters.Current() should come from config when CLI option is absent")
	})

	t.Run("ANSI enabled by default, disabled by --color=none", func(t *testing.T) {
		// Default (no flags) → enableANSI is true
		p1 := newPeco()
		require.NoError(t, p1.ApplyConfig(CLIOptions{}), "p.ApplyConfig should succeed")
		require.True(t, p1.enableANSI, "p.enableANSI should be true by default")

		// --color=none → enableANSI is false
		p2 := newPeco()
		require.NoError(t, p2.ApplyConfig(CLIOptions{OptColor: config.ColorModeNone}), "p.ApplyConfig should succeed")
		require.False(t, p2.enableANSI, "p.enableANSI should be false when OptColor is 'none'")

		// --color=auto → enableANSI is true
		p3 := newPeco()
		require.NoError(t, p3.ApplyConfig(CLIOptions{OptColor: config.ColorModeAuto}), "p.ApplyConfig should succeed")
		require.True(t, p3.enableANSI, "p.enableANSI should be true when OptColor is 'auto'")
	})

	t.Run("Invalid --color value is rejected", func(t *testing.T) {
		var c config.ColorMode
		err := c.UnmarshalFlag("bogus")
		require.Error(t, err, "invalid --color value should be rejected")
		require.Contains(t, err.Error(), "bogus")
	})
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
		require.Fail(t, "timeout reached")
		return
	case err := <-resultCh:
		require.True(t, util.IsCollectResultsError(err), "isCollectResultsError")
		p.PrintResults()
	}

	require.Equal(t, "foo\n", out.String(), "output should match")
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

	require.Equal(t, curbuf.Size(), 1, "There should be one element in buffer")

	for i := range curbuf.Size() {
		_, err := curbuf.LineAt(i)
		require.NoError(t, err, "LineAt(%d) should succeed", i)
	}

	require.Equal(t, "bar\n", buf.String(), "output should match")
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
			require.Fail(t, "timeout reached")
			return
		case err := <-resultCh:
			require.True(t, util.IsIgnorableError(err), "error should be ignorable")
			st, ok := util.GetExitStatus(err)
			require.True(t, ok, "error should have exit status")
			require.Equal(t, 1, st, "exit status should be 1")
		}

		require.Empty(t, out.String(), "output should be empty")
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
					require.Fail(t, "--exit-0 should not trigger when input is non-empty")
				}
			}
		}
	})
}

// runPecoSelectAll is a helper that runs peco in a goroutine and waits for
// it to complete. It uses a simple buffered channel to avoid goroutine
// scheduling races between the result send and the context cancellation.
func runPecoSelectAll(t *testing.T, p *Peco, ctx context.Context) { //nolint:revive
	t.Helper()

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- p.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		require.Fail(t, "timeout reached")
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
			require.Fail(t, "timeout reached")
			return
		case err := <-resultCh:
			require.True(t, util.IsCollectResultsError(err), "isCollectResultsError")
			p.PrintResults()
		}

		require.Equal(t, "oo\nfoo\n", out.String(), "output should match")
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
			require.Fail(t, "timeout reached")
			return
		case err := <-resultCh:
			require.True(t, util.IsCollectResultsError(err), "isCollectResultsError")
			p.PrintResults()
		}

		require.Equal(t, "oo\n", out.String(), "output should match")
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
			require.Fail(t, "Done channel should be closed after MarkComplete")
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
			require.Fail(t, "Done channel should be closed after MarkComplete")
		}
	})

	t.Run("works after Reset", func(t *testing.T) {
		mb := NewMemoryBuffer(0)
		mb.MarkComplete()

		mb.Reset()

		// After reset, done should be a new open channel
		select {
		case <-mb.Done():
			require.Fail(t, "Done channel should not be closed after Reset")
		default:
			// expected
		}

		// MarkComplete should work again on the new channel
		mb.MarkComplete()

		select {
		case <-mb.Done():
			// expected
		default:
			require.Fail(t, "Done channel should be closed after second MarkComplete")
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
	p.queryExec.delay = 5 * time.Second
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

	// Wait for the input loop to process the keystroke and create the timer
	require.Eventually(t, func() bool {
		p.queryExec.mutex.Lock()
		defer p.queryExec.mutex.Unlock()
		return p.queryExec.timer != nil
	}, 5*time.Second, 10*time.Millisecond, "queryExec.timer should have been created")

	// Cancel the context (simulating program exit)
	cancel()

	// Wait for Run to return
	<-waitCh

	// After Run returns, the timer should have been stopped.
	p.queryExec.mutex.Lock()
	timerAfterCancel := p.queryExec.timer
	p.queryExec.mutex.Unlock()
	require.Nil(t, timerAfterCancel, "queryExec.timer should be nil after cancellation")
}

// TestCancelFuncDataRace verifies that concurrent calls to Exit() and
// reads of Err() do not race with Run()'s write to cancelFunc. Without
// proper mutex protection on p.cancelFunc and p.err, the race detector
// flags this as a data race.
func TestCancelFuncDataRace(t *testing.T) {
	p := newPeco()
	p.Stdin = bytes.NewBufferString("foo\nbar\nbaz\n")
	var out bytes.Buffer
	p.Stdout = &out

	ctx := t.Context()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- p.Run(ctx)
	}()

	// Wait for peco to be ready (cancelFunc has been set by now)
	<-p.Ready()

	// Launch several goroutines that concurrently call Exit() and Err().
	// Under the race detector, unprotected access to p.cancelFunc and
	// p.err would be flagged.
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_ = p.Err()
		})
	}
	for i := range 5 {
		wg.Go(func() {
			p.Exit(fmt.Errorf("exit-%d", i))
		})
	}
	wg.Wait()

	// One of the Exit calls should have cancelled the context.
	select {
	case err := <-waitCh:
		// err could be any of the "exit-N" errors; just verify it's non-nil
		require.Error(t, err, "Run should return an error after Exit")
	case <-time.After(5 * time.Second):
		require.Fail(t, "timeout waiting for Run to return")
	}
}

func TestSelect1WithQuery(t *testing.T) {
	// --select-1 --query should auto-select when query narrows to exactly 1 match
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p := newPeco()
	p.Argv = []string{"--select-1", "--query", "bar"}
	p.Stdin = bytes.NewBufferString("foo\nbar\nbaz\n")
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
		require.Fail(t, "timeout: --select-1 --query bar should have auto-selected")
	case err := <-resultCh:
		require.True(t, util.IsCollectResultsError(err), "expected collectResultsError")
		p.PrintResults()
	}

	require.Equal(t, "bar\n", out.String(), "output should be the single matching line")
}

func TestWaitAndCall(t *testing.T) {
	t.Run("fires callback after timeout", func(t *testing.T) {
		p := newPeco()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		called := make(chan struct{})
		start := time.Now()
		go func() {
			p.waitAndCall(ctx, func() { close(called) })
		}()

		select {
		case <-called:
			elapsed := time.Since(start)
			require.True(t, elapsed >= 2*time.Second, "should wait at least 2s (got %v)", elapsed)
		case <-time.After(5 * time.Second):
			require.Fail(t, "callback was not fired within 5s")
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		p := newPeco()

		ctx, cancel := context.WithCancel(context.Background())

		called := false
		done := make(chan struct{})
		go func() {
			p.waitAndCall(ctx, func() { called = true })
			close(done)
		}()

		// Cancel quickly — before the 2s timer fires
		time.Sleep(200 * time.Millisecond)
		cancel()

		select {
		case <-done:
			require.False(t, called, "callback should NOT fire after context cancellation")
		case <-time.After(5 * time.Second):
			require.Fail(t, "waitAndCall did not return after context cancellation")
		}
	})
}

// TestMouseClickToggleSelection verifies that mouse events flow through
// the full pipeline: InjectMouse → tcellEventToEvent → input loop →
// keymap dispatch → action. This is an integration test for the mouse
// support restored after the tcell migration.
func TestMouseClickToggleSelection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p := newPeco()
	p.Argv = []string{}
	p.Stdin = bytes.NewBufferString("alpha\nbeta\ngamma\n")
	p.config.Keymap = map[string]string{
		"MouseLeft": "peco.ToggleSelectionAndSelectNext",
	}
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

	// Click to select first line, then press Enter to finish
	time.AfterFunc(100*time.Millisecond, func() {
		p.screen.SendEvent(Event{Type: EventKey, Key: keyseq.MouseLeft})
	})
	time.AfterFunc(200*time.Millisecond, func() {
		p.screen.SendEvent(Event{Type: EventKey, Key: keyseq.KeyEnter})
	})

	select {
	case <-ctx.Done():
		require.Fail(t, "timeout reached")
	case err := <-resultCh:
		require.True(t, util.IsCollectResultsError(err), "isCollectResultsError")
		p.PrintResults()
	}

	require.Equal(t, "alpha\n", out.String(), "mouse click should have selected first line")
}
