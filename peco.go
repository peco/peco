//go:generate stringer -type VerticalAnchor -output vertical_anchor_gen.go .

package peco

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"
	"unicode/utf8"

	"context"

	"github.com/google/btree"
	"github.com/lestrrat-go/pdebug"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/peco/peco/sig"
)

const version = "v0.5.11"

type ignorableError struct {
	err error
}

func (e ignorableError) Ignorable() bool { return true }
func (e ignorableError) Unwrap() error {
	return e.err
}
func (e ignorableError) Error() string {
	return e.err.Error()
}
func makeIgnorable(err error) error {
	return &ignorableError{err: err}
}

type exitStatusError struct {
	err    error
	status int
}

func (e exitStatusError) Error() string {
	return e.err.Error()
}
func (e exitStatusError) Unwrap() error {
	return e.err
}
func (e exitStatusError) ExitStatus() int {
	return e.status
}

func setExitStatus(err error, status int) error {
	return &exitStatusError{err: err, status: status}
}

// Inputseq is a list of keys that the user typed
type Inputseq []string

func (is *Inputseq) Add(s string) {
	*is = append(*is, s)
}

func (is Inputseq) KeyNames() []string {
	return is
}

func (is Inputseq) Len() int {
	return len(is)
}

func (is *Inputseq) Reset() {
	*is = []string(nil)
}

func newIDGen() *idgen {
	return &idgen{
		ch: make(chan uint64),
	}
}

func (ig *idgen) Run(ctx context.Context) {
	var i uint64
	for ; ; i++ {
		select {
		case <-ctx.Done():
			return
		case ig.ch <- i:
		}

		if i >= uint64(1<<63)-1 {
			// If this happens, it's a disaster, but what can we do...
			i = 0
		}
	}
}

func (ig *idgen) Next() uint64 {
	return <-ig.ch
}

func New() *Peco {
	return &Peco{
		Argv:              os.Args,
		Stderr:            os.Stderr,
		Stdin:             os.Stdin,
		Stdout:            os.Stdout,
		currentLineBuffer: NewMemoryBuffer(0), // XXX revisit this
		idgen:             newIDGen(),
		queryExec:         QueryExecState{delay: 50 * time.Millisecond},
		readyCh:           make(chan struct{}),
		readConfigFn:      readConfig,
		screen:            NewTcellScreen(),
		selection:         NewSelection(),
		maxScanBufferSize: bufio.MaxScanTokenSize,
	}
}

func (p *Peco) Ready() <-chan struct{} {
	return p.readyCh
}

func (p *Peco) Screen() Screen {
	return p.screen
}

func (p *Peco) Styles() *StyleSet {
	return &p.styles
}

func (p *Peco) Use256Color() bool {
	return p.use256Color
}

func (p *Peco) Prompt() string {
	return p.prompt
}

func (p *Peco) SelectionPrefix() string {
	return p.selectionPrefix
}

func (p *Peco) SuppressStatusMsg() bool {
	return p.config.SuppressStatusMsg
}

func (p *Peco) Inputseq() *Inputseq {
	return &p.inputseq
}

func (p *Peco) LayoutType() string {
	return p.layoutType
}

func (p *Peco) Location() *Location {
	return &p.location
}

func (p *Peco) ResultCh() chan line.Line {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.resultCh
}

func (p *Peco) SetResultCh(ch chan line.Line) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.resultCh = ch
}

func (p *Peco) Selection() *Selection {
	return p.selection
}

func (s RangeStart) Valid() bool {
	return s.valid
}

func (s RangeStart) Value() int {
	return s.val
}

func (s *RangeStart) SetValue(n int) {
	s.val = n
	s.valid = true
}

func (s *RangeStart) Reset() {
	s.valid = false
}

func (p *Peco) SelectionRangeStart() *RangeStart {
	return &p.selectionRangeStart
}

func (p *Peco) SingleKeyJump() *SingleKeyJumpState {
	return &p.singleKeyJump
}

func (p *Peco) ToggleSingleKeyJumpMode(ctx context.Context) {
	p.singleKeyJump.mode = !p.singleKeyJump.mode
	go p.Hub().SendDraw(ctx, &hub.DrawOptions{DisableCache: true})
}

func (p *Peco) Source() pipeline.Source {
	return p.source
}

func (p *Peco) Frozen() *FrozenState {
	return &p.frozen
}

func (p *Peco) Zoom() *ZoomState {
	return &p.zoom
}

// setCurrentLineBufferNoNotify sets the current line buffer under p.mutex
// without sending a draw event. Used by ZoomIn/ZoomOut where the caller
// manages draw notifications.
func (p *Peco) setCurrentLineBufferNoNotify(b Buffer) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.currentLineBuffer = b
}

func (p *Peco) Filters() *filter.Set {
	return &p.filters
}

func (p *Peco) Query() *Query {
	return &p.query
}

func (p *Peco) QueryExec() *QueryExecState {
	return &p.queryExec
}

func (p *Peco) Caret() *Caret {
	return &p.caret
}

func (p *Peco) Hub() MessageHub {
	return p.hub
}

func (p *Peco) Err() error {
	return p.err
}

func (p *Peco) Exit(err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.Exit (err = %s)", err)
		defer g.End()
	}
	p.err = err
	if cf := p.cancelFunc; cf != nil {
		cf()
	}
}

func (p *Peco) Keymap() Keymap {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.keymap
}

func (p *Peco) Setup() (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.Setup").BindError(&err)
		defer g.End()
	}

	if err := p.config.Init(); err != nil {
		return fmt.Errorf("failed to initialize config: %w", err)
	}

	var opts CLIOptions
	if err := p.parseCommandLine(&opts, &p.args, p.Argv); err != nil {
		return fmt.Errorf("failed to parse command line: %w", err)
	}

	// Read config
	if err := p.readConfigFn(&p.config, opts.OptRcfile); err != nil {
		return fmt.Errorf("failed to setup configuration: %w", err)
	}

	// Take Args, Config, Options, and apply the configuration to
	// the peco object
	if err := p.ApplyConfig(opts); err != nil {
		return fmt.Errorf("failed to apply configuration: %w", err)
	}

	// XXX p.Keymap et al should be initialized around here
	p.hub = hub.New(5)

	return nil
}

func (p *Peco) selectOneAndExitIfPossible() {
	// If we have only one line, we just want to bail out
	// printing that one line as the result
	if b := p.CurrentLineBuffer(); b.Size() == 1 {
		if l, err := b.LineAt(0); err == nil {
			ch := make(chan line.Line)
			p.SetResultCh(ch)
			p.Exit(collectResultsError{})
			ch <- l
			close(ch)
		}
	}
}

func (p *Peco) exitZeroIfPossible() {
	if p.CurrentLineBuffer().Size() == 0 {
		p.Exit(setExitStatus(makeIgnorable(errors.New("no input, exiting")), 1))
	}
}

func (p *Peco) selectAllAndExitIfPossible() {
	b := p.CurrentLineBuffer()
	selection := p.Selection()
	for i := range b.Size() {
		if l, err := b.LineAt(i); err == nil {
			selection.Add(l)
		}
	}
	p.Exit(collectResultsError{})
}

// startComponents waits for the source to be ready, then initializes the
// screen and starts the Input, View, and Filter goroutines.
func (p *Peco) startComponents(ctx context.Context, cancel func()) {
	<-p.source.Ready()
	// screen.Init must be called within Run() because we
	// want to make sure to call screen.Close() after getting
	// out of Run()
	if err := p.screen.Init(&p.config); err != nil {
		p.Exit(fmt.Errorf("failed to initialize screen: %w", err))
		return
	}
	go func() { _ = NewInput(p, p.Keymap(), p.screen.PollEvent(ctx, &p.config)).Loop(ctx, cancel) }()
	v, err := NewView(p)
	if err != nil {
		p.Exit(fmt.Errorf("failed to create view: %w", err))
		return
	}
	go func() { _ = v.Loop(ctx, cancel) }()
	go func() { _ = NewFilter(p).Loop(ctx, cancel) }()
}

// startEarlyExitHandlers launches goroutines that handle --select-1,
// --exit-0, and --select-all modes, each waiting for the source to
// finish reading before checking conditions.
func (p *Peco) startEarlyExitHandlers() {
	// If this is enabled, we need to check if we have 1 line only
	// in the buffer. If we do, we select that line and bail out
	if p.selectOneAndExit {
		go func() {
			// Wait till source has read all lines. We should not wait
			// source.Ready(), because Ready returns as soon as we get
			// a line, where as SetupDone waits until we're completely
			// done reading the input
			<-p.source.SetupDone()
			p.selectOneAndExitIfPossible()
		}()
	}

	// If this is enabled, exit immediately with status 1 when input is empty
	if p.exitZeroAndExit {
		go func() {
			<-p.source.SetupDone()
			p.exitZeroIfPossible()
		}()
	}

	// If --select-all is enabled and there is no query, select all lines
	// from the source and exit immediately
	if p.selectAllAndExit && p.initialQuery == "" {
		go func() {
			<-p.source.SetupDone()
			p.selectAllAndExitIfPossible()
		}()
	}
}

// setupInitialQuery sets the query text and caret position from the
// --query flag, then triggers query execution if there is a query.
func (p *Peco) setupInitialQuery(ctx context.Context) {
	// This has tobe AFTER close(p.readyCh), otherwise the query is
	// ignored by us (queries are not run until peco thinks it's ready)
	if q := p.initialQuery; q != "" {
		p.Query().Set(q)
		p.Caret().SetPos(utf8.RuneCountInString(q))
	}

	if p.Query().Len() > 0 {
		go func() {
			<-p.source.Ready()

			// iff p.selectOneAndExit is true, we should check after exec query is run
			// if we only have one item
			if p.selectOneAndExit {
				p.ExecQuery(ctx, p.selectOneAndExitIfPossible)
			} else if p.selectAllAndExit {
				p.ExecQuery(ctx, p.selectAllAndExitIfPossible)
			} else {
				p.ExecQuery(ctx, nil)
			}
		}()
	}
}

func (p *Peco) Run(ctx context.Context) (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.Run").BindError(&err)
		defer g.End()
	}

	// do this only once
	var readyOnce sync.Once
	defer readyOnce.Do(func() { close(p.readyCh) })

	if err := p.Setup(); err != nil {
		return fmt.Errorf("failed to setup peco: %w", err)
	}

	var _cancelOnce sync.Once
	var _cancel func()
	ctx, _cancel = context.WithCancel(ctx)
	cancel := func() {
		_cancelOnce.Do(func() {
			if pdebug.Enabled {
				pdebug.Printf("Peco.Run cancel called")
			}
			_cancel()
		})
	}

	// start the ID generator
	go p.idgen.Run(ctx)

	// remember this cancel func so p.Exit works (XXX requires locking?)
	p.cancelFunc = cancel

	sigH := sig.New(sig.ReceivedHandlerFunc(func(sig os.Signal) {
		p.Exit(errors.New("received signal: " + sig.String()))
	}))

	go func() { _ = sigH.Loop(ctx, cancel) }()

	// SetupSource is done AFTER other components are ready, otherwise
	// we can't draw onto the screen while we are reading a really big
	// buffer.
	// Setup source buffer
	src, err := p.SetupSource(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup input source: %w", err)
	}
	p.source = src

	// If --height is specified, use InlineScreen instead of the default TcellScreen
	if p.heightSpec != nil {
		p.screen = NewInlineScreen(*p.heightSpec)
	}

	go p.startComponents(ctx, cancel)
	defer p.screen.Close()

	if p.Query().Len() <= 0 {
		// Re-set the source only if there are no queries
		p.ResetCurrentLineBuffer(ctx)
	}

	if pdebug.Enabled {
		pdebug.Printf("peco is now ready, go go!")
	}

	p.startEarlyExitHandlers()

	readyOnce.Do(func() { close(p.readyCh) })

	p.setupInitialQuery(ctx)

	// Alright, done everything we need to do automatically. We'll let
	// the user play with peco, and when we receive notification to
	// bail out, the context should be canceled appropriately
	<-ctx.Done()

	// Stop any pending query exec timer to prevent the callback
	// from firing after program state is torn down.
	p.queryExec.StopTimer()

	// ...and we return any errors that we might have been informed about.
	return p.Err()
}

func (p *Peco) parseCommandLine(opts *CLIOptions, args *[]string, argv []string) error {
	remaining, err := opts.parse(argv)
	if err != nil {
		return fmt.Errorf("failed to parse command line options: %w", err)
	}

	if opts.OptHelp {
		if _, err := p.Stdout.Write(opts.help()); err != nil {
			return fmt.Errorf("failed to write help: %w", err)
		}
		return makeIgnorable(errors.New("user asked to show help message"))
	}

	if opts.OptVersion {
		fmt.Fprintf(p.Stdout, "peco version %s (built with %s)\n", version, runtime.Version())
		return makeIgnorable(errors.New("user asked to show version"))
	}

	if opts.OptRcfile == "" {
		if file, err := LocateRcfile(locateRcfileIn); err == nil {
			opts.OptRcfile = file
		}
	}

	*args = remaining

	return nil
}

func (p *Peco) SetupSource(ctx context.Context) (s *Source, err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.SetupSource").BindError(&err)
		defer g.End()
	}

	var in io.Reader
	var filename string
	var isInfinite bool
	switch {
	case len(p.args) > 1:
		f, err := os.Open(p.args[1])
		if err != nil {
			return nil, fmt.Errorf("failed to open file for input: %w", err)
		}
		if pdebug.Enabled {
			pdebug.Printf("Using %s as input", p.args[1])
		}
		in = f
		filename = p.args[1]
	case !util.IsTty(p.Stdin):
		if pdebug.Enabled {
			pdebug.Printf("Using p.Stdin as input")
		}
		in = p.Stdin
		filename = `-`
		// XXX we detect that this is potentially an "infinite" source if
		// the input is coming from Stdin. This is important b/c we need to
		// know NOT to use batch mode processing when the incoming source
		// is never-ending
		isInfinite = true
	default:
		return nil, errors.New("you must supply something to work with via filename or stdin")
	}

	src := NewSource(filename, in, isInfinite, p.idgen, p.bufferSize, p.enableSep, p.enableANSI)

	// Block until we receive something from `in`
	if pdebug.Enabled {
		pdebug.Printf("Blocking until we read something in source...")
	}

	go src.Setup(ctx, p)
	<-src.Ready()

	return src, nil
}

func readConfig(cfg *Config, filename string) error {
	if filename != "" {
		if err := cfg.ReadFilename(filename); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	return nil
}

func (p *Peco) ApplyConfig(opts CLIOptions) error {
	// If layoutType is not set and is set in the config, set it
	if p.layoutType == "" {
		if v := p.config.Layout; v != "" {
			p.layoutType = v
		} else {
			p.layoutType = DefaultLayoutType
		}
	}

	p.maxScanBufferSize = 256
	if v := p.config.MaxScanBufferSize; v > 0 {
		p.maxScanBufferSize = v
	}

	if v := opts.OptExec; len(v) > 0 {
		p.execOnFinish = v
	}

	p.enableSep = opts.OptEnableNullSep
	p.enableANSI = opts.OptANSI || p.config.ANSI

	if i := opts.OptInitialIndex; i >= 0 {
		p.Location().SetLineNumber(i)
	}

	if v := opts.OptLayout; v != "" {
		p.layoutType = v
	}

	p.prompt = p.config.Prompt
	if v := opts.OptPrompt; len(v) > 0 {
		p.prompt = v
	} else if v := p.config.Prompt; len(v) > 0 {
		p.prompt = v
	}

	p.use256Color = p.config.Use256Color

	p.onCancel = p.config.OnCancel
	if p.onCancel == "" {
		p.onCancel = OnCancelSuccess
	}
	if opts.OptOnCancel != "" {
		if err := p.onCancel.UnmarshalText([]byte(opts.OptOnCancel)); err != nil {
			return fmt.Errorf("invalid --on-cancel value: %w", err)
		}
	}
	p.bufferSize = opts.OptBufferSize
	if v := opts.OptSelectionPrefix; len(v) > 0 {
		p.selectionPrefix = v
	} else {
		p.selectionPrefix = p.config.SelectionPrefix
	}
	p.selectOneAndExit = opts.OptSelect1
	p.exitZeroAndExit = opts.OptExitZero
	p.selectAllAndExit = opts.OptSelectAll
	p.printQuery = opts.OptPrintQuery
	p.initialQuery = opts.OptQuery
	p.initialFilter = opts.OptInitialFilter
	if len(p.initialFilter) <= 0 {
		p.initialFilter = p.config.InitialFilter
	}
	p.fuzzyLongestSort = p.config.FuzzyLongestSort

	// Height: CLI option overrides config
	var heightStr string
	if v := opts.OptHeight; v != "" {
		heightStr = v
	} else if v := p.config.Height; v != "" {
		heightStr = v
	}
	if heightStr != "" {
		spec, err := ParseHeightSpec(heightStr)
		if err != nil {
			return fmt.Errorf("failed to parse height specification: %w", err)
		}
		p.heightSpec = &spec
	}

	if err := p.populateFilters(); err != nil {
		return fmt.Errorf("failed to populate filters: %w", err)
	}

	if err := p.populateKeymap(); err != nil {
		return fmt.Errorf("failed to populate keymap: %w", err)
	}

	if err := p.populateStyles(); err != nil {
		return fmt.Errorf("failed to populate styles: %w", err)
	}

	if err := p.populateInitialFilter(); err != nil {
		return fmt.Errorf("failed to populate initial filter: %w", err)
	}

	if err := p.populateSingleKeyJump(); err != nil {
		return fmt.Errorf("failed to populate single key jump configuration: %w", err)
	}

	return nil
}

func (p *Peco) populateInitialFilter() error {
	if v := p.initialFilter; len(v) > 0 {
		if err := p.filters.SetCurrentByName(v); err != nil {
			return fmt.Errorf("failed to set filter: %w", err)
		}
	}
	return nil
}

func (p *Peco) populateSingleKeyJump() error { //nolint:unparam
	p.singleKeyJump.showPrefix = p.config.SingleKeyJump.ShowPrefix

	jumpMap := make(map[rune]uint)
	chrs := "asdfghjklzxcvbnmqwertyuiop"
	for i := range len(chrs) {
		jumpMap[rune(chrs[i])] = uint(i)
	}
	p.singleKeyJump.prefixMap = jumpMap

	p.singleKeyJump.prefixes = make([]rune, len(jumpMap))
	for k, v := range p.singleKeyJump.prefixMap {
		p.singleKeyJump.prefixes[v] = k
	}
	return nil
}

func (p *Peco) populateFilters() error { //nolint:unparam
	_ = p.filters.Add(filter.NewIgnoreCase())
	_ = p.filters.Add(filter.NewCaseSensitive())
	_ = p.filters.Add(filter.NewSmartCase())
	_ = p.filters.Add(filter.NewIRegexp())
	_ = p.filters.Add(filter.NewRegexp())
	_ = p.filters.Add(filter.NewFuzzy(p.fuzzyLongestSort))

	for name, c := range p.config.CustomFilter {
		f := filter.NewExternalCmd(name, c.Cmd, c.Args, c.BufferThreshold, p.idgen, p.enableSep)
		_ = p.filters.Add(f)
	}

	return nil
}

func (p *Peco) populateKeymap() error {
	// Create a new keymap object
	k := NewKeymap(p.config.Keymap, p.config.Action)
	if err := k.ApplyKeybinding(); err != nil {
		return fmt.Errorf("failed to apply key bindings: %w", err)
	}
	p.mutex.Lock()
	p.keymap = k
	p.mutex.Unlock()
	return nil
}

func (p *Peco) populateStyles() error { //nolint:unparam
	p.styles = p.config.Style
	return nil
}

func (p *Peco) CurrentLineBuffer() Buffer {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.currentLineBuffer
}

func (p *Peco) SetCurrentLineBuffer(ctx context.Context, b Buffer) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.SetCurrentLineBuffer %s", reflect.TypeOf(b).String())
		defer g.End()
	}
	p.currentLineBuffer = b
	go p.Hub().SendDraw(ctx, nil)
}

func (p *Peco) ResetCurrentLineBuffer(ctx context.Context) {
	if fs := p.Frozen().Source(); fs != nil {
		p.SetCurrentLineBuffer(ctx, fs)
	} else {
		p.SetCurrentLineBuffer(ctx, p.source)
	}
}

func (p *Peco) sendQuery(ctx context.Context, q string, nextFunc func()) {
	if pdebug.Enabled {
		g := pdebug.Marker("sending query to filter goroutine (q=%v, isInfinite=%t)", q, p.source.IsInfinite())
		defer g.End()
	}

	if p.source.IsInfinite() {
		// If the source is a stream, we can't do batch mode, and hence
		// we can't guarantee proper timing. But... okay, we simulate
		// something like it
		p.Hub().SendQuery(ctx, q)
		if nextFunc != nil {
			time.AfterFunc(time.Second, nextFunc)
		}
	} else {
		// No delay, execute immediately
		p.Hub().Batch(ctx, func(ctx context.Context) {
			p.Hub().SendQuery(ctx, q)
			if nextFunc != nil {
				nextFunc()
			}
		}, false)
	}
}

// ExecQuery executes the query, taking in consideration things like the
// exec-delay, and user's multiple successive inputs in a very short span
//
// if nextFunc is non-nil, then nextFunc is executed after the query is
// executed
func (p *Peco) ExecQuery(ctx context.Context, nextFunc func()) bool {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.ExecQuery")
		defer g.End()
	}

	msgHub := p.Hub()

	select {
	case <-p.Ready():
	default:
		if pdebug.Enabled {
			pdebug.Printf("peco is not ready yet, ignoring.")
		}
		return false
	}

	// If this is an empty query, reset the display to show
	// the raw source buffer
	q := p.Query()
	if q.Len() <= 0 {
		if pdebug.Enabled {
			pdebug.Printf("empty query, reset buffer")
		}
		p.ResetCurrentLineBuffer(ctx)

		msgHub.Batch(ctx, func(ctx context.Context) {
			msgHub.SendDraw(ctx, &hub.DrawOptions{DisableCache: true})
			if nextFunc != nil {
				nextFunc()
			}
		}, false)
		return true
	}

	delay := p.queryExec.delay
	if delay <= 0 {
		if pdebug.Enabled {
			pdebug.Printf("sending query (immediate)")
		}

		p.sendQuery(ctx, q.String(), nextFunc)
		return true
	}

	p.queryExec.mutex.Lock()
	defer p.queryExec.mutex.Unlock()

	if p.queryExec.timer != nil {
		if pdebug.Enabled {
			pdebug.Printf("timer is non-nil")
		}
		return true
	}

	// Wait $delay millisecs before sending the query
	// if a new input comes in, batch them up
	if pdebug.Enabled {
		pdebug.Printf("sending query (with delay)")
	}
	p.queryExec.timer = time.AfterFunc(delay, func() {
		// Acquire the mutex first to synchronize with QueryExec.StopTimer.
		// If StopTimer already ran (during shutdown), the timer
		// field will be nil and we must not proceed — the receivers on
		// hub channels may have already exited.
		p.queryExec.mutex.Lock()
		if p.queryExec.timer == nil {
			p.queryExec.mutex.Unlock()
			return
		}
		p.queryExec.timer = nil
		p.queryExec.mutex.Unlock()

		if pdebug.Enabled {
			pdebug.Printf("delayed query sent")
		}
		p.sendQuery(ctx, q.String(), nextFunc)
		if pdebug.Enabled {
			pdebug.Printf("delayed query executed")
		}
	})
	return true
}

func (p *Peco) PrintResults() {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.PrintResults")
		defer g.End()
	}
	selection := p.Selection()
	if selection.Len() == 0 {
		if l, err := p.CurrentLineBuffer().LineAt(p.Location().LineNumber()); err == nil {
			selection.Add(l)
		}
	}
	p.SetResultCh(make(chan line.Line))
	go func() {
		defer close(p.resultCh)
		p.selection.Ascend(func(it btree.Item) bool {
			l, ok := it.(line.Line)
			if !ok {
				return true
			}
			p.ResultCh() <- l
			return true
		})
	}()

	var buf bytes.Buffer

	if pdebug.Enabled {
		pdebug.Printf("--print-query was %t", p.printQuery)
	}
	if p.printQuery {
		buf.WriteString(p.Query().String())
		buf.WriteByte('\n')
	}
	for line := range p.ResultCh() {
		buf.WriteString(line.Output())
		buf.WriteByte('\n')
	}
	_, _ = p.Stdout.Write(buf.Bytes())
}
