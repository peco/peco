//go:generate stringer -type PagingRequestType -output stringer_paging_request_type.go .
//go:generate stringer -type VerticalAnchor    -output stringer_vertical_anchor.go     .

package peco

import (
	"bufio"
	"bytes"
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
	"github.com/lestrrat-go/pdebug/v2"
	"github.com/peco/peco/buffer"
	"github.com/peco/peco/filter"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/location"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/line"
	"github.com/peco/peco/pipeline"
	"github.com/peco/peco/query"
	"github.com/peco/peco/sig"
	"github.com/peco/peco/ui"
	"github.com/pkg/errors"
)

const version = "v0.5.7"

type errIgnorable struct {
	err error
}

func (e errIgnorable) Ignorable() bool { return true }
func (e errIgnorable) Cause() error {
	return e.err
}

func (e errIgnorable) Error() string {
	return e.err.Error()
}

func makeIgnorable(err error) error {
	return &errIgnorable{err: err}
}

type errWithExitStatus struct {
	err    error
	status int
}

func (e errWithExitStatus) Error() string {
	return e.err.Error()
}
func (e errWithExitStatus) Cause() error {
	return e.err
}
func (e errWithExitStatus) ExitStatus() int {
	return e.status
}

func setExitStatus(err error, status int) error {
	return &errWithExitStatus{err: err, status: status}
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
		currentLineBuffer: buffer.NewMemory(), // XXX revisit this
		idgen:             newIDGen(),
		query:             query.New(),
		queryExecDelay:    50 * time.Millisecond,
		readyCh:           make(chan struct{}),
		screen:            ui.NewTermbox(),
		selection:         ui.NewSelection(),
		maxScanBufferSize: bufio.MaxScanTokenSize,
	}
}

func (p *Peco) AnchorPosition() int {
	return p.anchor.AnchorPosition()
}

func (p *Peco) Ready() <-chan struct{} {
	return p.readyCh
}

func (p *Peco) SelectionPrefix() string {
	return p.selectionPrefix
}

func (p *Peco) Screen() ui.Screen {
	return p.screen
}

func (p *Peco) Styles() *ui.StyleSet {
	return p.styles
}

func (p *Peco) Prompt() string {
	return p.prompt
}

func (p *Peco) Inputseq() *Inputseq {
	return &p.inputseq
}

func (p *Peco) LayoutType() string {
	return p.layoutType
}

func (p *Peco) Location() *location.Location {
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

func (p *Peco) Selection() *ui.Selection {
	return p.selection
}

func (p *Peco) SelectionRangeStart() *ui.RangeStart {
	return &p.selectionRangeStart
}

func (p *Peco) SingleKeyJumpShowPrefix() bool {
	return p.singleKeyJumpShowPrefix
}

func (p *Peco) SingleKeyJumpPrefixes() []rune {
	return p.singleKeyJumpPrefixes
}

func (p *Peco) SingleKeyJumpMode() bool {
	return p.singleKeyJumpMode
}

func (p *Peco) SetSingleKeyJumpMode(b bool) {
	p.singleKeyJumpMode = b
}

func (p *Peco) ToggleSingleKeyJumpMode() {
	p.singleKeyJumpMode = !p.singleKeyJumpMode
	go p.Hub().SendDraw(context.Background(), ui.WithLineCache(false))
}

func (p *Peco) SingleKeyJumpIndex(ch rune) (uint, bool) {
	n, ok := p.singleKeyJumpPrefixMap[ch]
	return n, ok
}

func (p *Peco) Source() pipeline.Source {
	return p.source
}

func (p *Peco) Filters() *filter.Set {
	return &p.filters
}

func (p *Peco) Query() *query.Query {
	return p.query
}

func (p *Peco) QueryExecDelay() time.Duration {
	return p.queryExecDelay
}

func (p *Peco) Caret() *ui.Caret {
	return &p.caret
}

func (p *Peco) Hub() MessageHub {
	return p.hub
}

func (p *Peco) Err() error {
	return p.err
}

func (p *Peco) Exit(ctx context.Context, err error) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "Peco.Exit (err = %s)", err)
		defer g.End()
	}
	p.err = err
	if cf := p.cancelFunc; cf != nil {
		cf()
	}
}

func (p *Peco) Keymap() Keymap {
	return p.keymap
}

func (p *Peco) Setup(ctx context.Context) (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "Peco.Setup").BindError(&err)
		defer g.End()
	}

	if err := p.config.Init(); err != nil {
		return errors.Wrap(err, "failed to initialize config")
	}

	var opts CLIOptions
	if err := p.parseCommandLine(&opts, &p.args, p.Argv); err != nil {
		return errors.Wrap(err, "failed to parse command line")
	}

	// Read config
	if !p.skipReadConfig { // This can only be set via test
		if err := readConfig(&p.config, opts.OptRcfile); err != nil {
			return errors.Wrap(err, "failed to setup configuration")
		}
	}

	// Take Args, Config, Options, and apply the configuration to
	// the peco object
	if err := p.ApplyConfig(opts); err != nil {
		return errors.Wrap(err, "failed to apply configuration")
	}

	// XXX p.Keymap et al should be initialized around here
	p.hub = hub.New(5)

	return nil
}

func (p *Peco) selectOneAndExitIfPossible(ctx context.Context) {
	// TODO: mutex
	// If we have only one line, we just want to bail out
	// printing that one line as the result
	if b := p.CurrentLineBuffer(); b.Size() == 1 {
		if l, err := b.LineAt(0); err == nil {
			p.resultCh = make(chan line.Line)
			p.Exit(ctx, errCollectResults{})

			select {
			case <-ctx.Done():
				return
			case p.resultCh <- l:
				close(p.resultCh)
			}

		}
	}
}

func (p *Peco) Run(ctx context.Context) (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "Peco.Run").BindError(&err)
		defer g.End()
	}

	// do this only once
	var readyOnce sync.Once
	defer readyOnce.Do(func() { close(p.readyCh) })

	if err := p.Setup(ctx); err != nil {
		return errors.Wrap(err, "failed to setup peco")
	}

	var _cancelOnce sync.Once
	var _cancel func()
	ctx, _cancel = context.WithCancel(ctx)
	cancel := func() {
		_cancelOnce.Do(func() {
			pdebug.Printf(ctx, "Peco.Run cancel called")
			_cancel()
		})
	}

	// start the ID generator
	go p.idgen.Run(ctx)

	// remember this cancel func so p.Exit works (XXX requires locking?)
	p.cancelFunc = cancel

	sigH := sig.New(sig.SigReceivedHandlerFunc(func(sig os.Signal) {
		p.Exit(ctx, errors.New("received signal: "+sig.String()))
	}))

	go func() { _ = sigH.Loop(ctx, cancel) }()

	// SetupSource is done AFTER other components are ready, otherwise
	// we can't draw onto the screen while we are reading a really big
	// buffer.
	// Setup source buffer
	src, err := p.SetupSource(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to setup input source")
	}
	p.source = src

	go func() {
		<-p.source.Ready()
		// screen.Init must be called within Run() because we
		// want to make sure to call screen.Close() after getting
		// out of Run()
		_ = p.screen.Init()
		go func() { _ = NewInput(p, p.Keymap(), p.screen.PollEvent(ctx)).Loop(ctx, cancel) }()
		go func() { _ = NewView(p).Loop(ctx, cancel) }()
		go func() { _ = NewFilter(p).Loop(ctx, cancel) }()
	}()
	defer p.screen.Close()

	if p.Query().Len() <= 0 {
		// Re-set the source only if there are no queries
		p.ResetCurrentLineBuffer()
	}

	if pdebug.Enabled {
		pdebug.Printf(ctx, "peco is now ready, go go go!")
	}

	// If this is enabled, we need to check if we have 1 line only
	// in the buffer. If we do, we select that line and bail out
	if p.selectOneAndExit {
		go func() {
			// Wait till source has read all lines. We should not wait
			// source.Ready(), because Ready returns as soon as we get
			// a line, where as SetupDone waits until we're completely
			// done reading the input
			<-p.source.SetupDone()
			p.selectOneAndExitIfPossible(ctx)
		}()
	}

	readyOnce.Do(func() { close(p.readyCh) })

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
				p.ExecQuery(p.selectOneAndExitIfPossible)
			} else {
				p.ExecQuery(nil)
			}
		}()
	}

	// Alright, done everything we need to do automatically. We'll let
	// the user play with peco, and when we receive notification to
	// bail out, the context should be canceled appropriately
	<-ctx.Done()

	// ...and we return any errors that we might have been informed about.
	return p.Err()
}

func (p *Peco) parseCommandLine(opts *CLIOptions, args *[]string, argv []string) error {
	remaining, err := opts.parse(argv)
	if err != nil {
		return errors.Wrap(err, "failed to parse command line options")
	}

	if opts.OptHelp {
		_, _ = p.Stdout.Write(opts.help())
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
		g := pdebug.Marker(ctx, "Peco.SetupSource").BindError(&err)
		defer g.End()
	}

	var in io.Reader
	var filename string
	var isInfinite bool
	switch {
	case len(p.args) > 1:
		f, err := os.Open(p.args[1])
		if err != nil {
			return nil, errors.Wrap(err, "failed to open file for input")
		}
		if pdebug.Enabled {
			pdebug.Printf(ctx, "Using %s as input", p.args[1])
		}
		in = f
		filename = p.args[1]
	case !util.IsTty(p.Stdin):
		if pdebug.Enabled {
			pdebug.Printf(ctx, "Using p.Stdin as input")
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

	src := NewSource(filename, in, isInfinite, p.idgen, p.bufferSize, p.enableSep)

	// Block until we receive something from `in`
	if pdebug.Enabled {
		pdebug.Printf(ctx, "Blocking until we read something in source...")
	}

	go src.Setup(ctx, p)
	<-src.Ready()

	return src, nil
}

func readConfig(cfg *Config, filename string) error {
	if filename != "" {
		if err := cfg.ReadFilename(filename); err != nil {
			return errors.Wrap(err, "failed to read config file")
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
			p.layoutType = ui.DefaultLayoutType
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

	p.onCancel = successKey
	if opts.OptOnCancel == errorKey || p.config.OnCancel == errorKey {
		p.onCancel = errorKey
	}
	p.bufferSize = opts.OptBufferSize
	if v := opts.OptSelectionPrefix; len(v) > 0 {
		p.selectionPrefix = v
	} else {
		p.selectionPrefix = p.config.SelectionPrefix
	}
	p.selectOneAndExit = opts.OptSelect1
	p.printQuery = opts.OptPrintQuery
	p.initialQuery = opts.OptQuery
	p.initialFilter = opts.OptInitialFilter
	if len(p.initialFilter) <= 0 {
		p.initialFilter = p.config.InitialFilter
	}
	if len(p.initialFilter) <= 0 {
		p.initialFilter = opts.OptInitialMatcher
	}

	if err := p.populateFilters(); err != nil {
		return errors.Wrap(err, "failed to populate filters")
	}

	if err := p.populateKeymap(); err != nil {
		return errors.Wrap(err, "failed to populate keymap")
	}

	if err := p.populateStyles(); err != nil {
		return errors.Wrap(err, "failed to populate styles")
	}

	if err := p.populateInitialFilter(); err != nil {
		return errors.Wrap(err, "failed to populate initial filter")
	}

	if err := p.populateSingleKeyJump(); err != nil {
		return errors.Wrap(err, "failed to populate single key jump configuration")
	}

	return nil
}

func (p *Peco) populateInitialFilter() error {
	if v := p.initialFilter; len(v) > 0 {
		if err := p.filters.SetCurrentByName(v); err != nil {
			return errors.Wrap(err, "failed to set filter")
		}
	}
	return nil
}

func (p *Peco) populateSingleKeyJump() error {
	p.singleKeyJumpShowPrefix = p.config.SingleKeyJump.ShowPrefix

	jumpMap := make(map[rune]uint)
	chrs := "asdfghjklzxcvbnmqwertyuiop"
	for i := 0; i < len(chrs); i++ {
		jumpMap[rune(chrs[i])] = uint(i)
	}
	p.singleKeyJumpPrefixMap = jumpMap

	p.singleKeyJumpPrefixes = make([]rune, len(jumpMap))
	for k, v := range p.singleKeyJumpPrefixMap {
		p.singleKeyJumpPrefixes[v] = k
	}
	return nil
}

func (p *Peco) populateFilters() error {
	filters := []filter.Filter{
		filter.NewIgnoreCase(),
		filter.NewCaseSensitive(),
		filter.NewSmartCase(),
		filter.NewRegexp(),
		filter.NewFuzzy(),
	}

	for name, c := range p.config.CustomFilter {
		f := filter.NewExternalCmd(name, c.Cmd, c.Args, c.BufferThreshold, p.idgen, p.enableSep)
		filters = append(filters, f)
	}

	for _, f := range filters {
		if err := p.filters.Add(f); err != nil {
			return errors.Wrap(err, `failed to add filter`)
		}
	}

	return nil
}

func (p *Peco) populateKeymap() error {
	// Create a new keymap object
	k := NewKeymap(p.config.Keymap, p.config.Action)
	if err := k.ApplyKeybinding(); err != nil {
		return errors.Wrap(err, "failed to apply key bindings")
	}
	p.keymap = k
	return nil
}

func (p *Peco) populateStyles() error {
	p.styles = p.config.Style
	return nil
}

func (p *Peco) CurrentLineBuffer() buffer.Buffer {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.currentLineBuffer
}

func (p *Peco) SetCurrentLineBuffer(b buffer.Buffer) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if pdebug.Enabled {
		g := pdebug.Marker(context.TODO(), "Peco.SetCurrentLineBuffer %s", reflect.TypeOf(b).String())
		defer g.End()
	}
	p.currentLineBuffer = b
	go p.Hub().SendDraw(context.Background(), nil)
}

func (p *Peco) ResetCurrentLineBuffer() {
	p.SetCurrentLineBuffer(p.source)
}

func (p *Peco) sendQuery(ctx context.Context, q string, nextFunc func(context.Context)) {
	if pdebug.Enabled {
		g := pdebug.Marker(ctx, "sending query to filter goroutine (q=%v, isInfinite=%t)", q, p.source.IsInfinite())
		defer g.End()
	}

	if p.source.IsInfinite() {
		// If the source is a stream, we can't do batch mode, and hence
		// we can't guarantee proper timing. But... okay, we simulate
		// something like it
		p.Hub().SendQuery(ctx, q)
		if nextFunc != nil {
			time.AfterFunc(time.Second, func() { nextFunc(ctx) })
		}
	} else {
		// No delay, execute immediately
		p.Hub().Batch(context.Background(), func(ctx context.Context) {
			p.Hub().SendQuery(ctx, q)
			if nextFunc != nil {
				nextFunc(ctx)
			}
		}, false)
	}
}

// ExecQuery executes the query, taking in consideration things like the
// exec-delay, and user's multiple successive inputs in a very short span
//
// if nextFunc is non-nil, then nextFunc is executed after the query is
// executed
func (p *Peco) ExecQuery(nextFunc func(context.Context)) bool {
	if pdebug.Enabled {
		g := pdebug.Marker(context.TODO(), "Peco.ExecQuery")
		defer g.End()
	}

	hub := p.Hub()

	select {
	case <-p.Ready():
	default:
		if pdebug.Enabled {
			pdebug.Printf(context.TODO(), "peco is not ready yet, ignoring.")
		}
		return false
	}

	// If this is an empty query, reset the display to show
	// the raw source buffer
	q := p.Query()
	if q.Len() <= 0 {
		if pdebug.Enabled {
			pdebug.Printf(context.TODO(), "empty query, reset buffer")
		}
		p.ResetCurrentLineBuffer()

		hub.Batch(context.Background(), func(ctx context.Context) {
			hub.SendDraw(ctx, ui.WithLineCache(false))
			if nextFunc != nil {
				nextFunc(ctx)
			}
		}, false)
		return true
	}

	delay := p.QueryExecDelay()
	if delay <= 0 {
		if pdebug.Enabled {
			pdebug.Printf(context.TODO(), "sending query (immediate)")
		}

		p.sendQuery(context.Background(), q.String(), nextFunc)
		return true
	}

	p.queryExecMutex.Lock()
	defer p.queryExecMutex.Unlock()

	if p.queryExecTimer != nil {
		if pdebug.Enabled {
			pdebug.Printf(context.TODO(), "timer is non-nil")
		}
		return true
	}

	// Wait $delay millisecs before sending the query
	// if a new input comes in, batch them up
	if pdebug.Enabled {
		pdebug.Printf(context.TODO(), "sending query (with delay)")
	}
	p.queryExecTimer = time.AfterFunc(delay, func() {
		if pdebug.Enabled {
			pdebug.Printf(context.TODO(), "delayed query sent")
		}
		p.sendQuery(context.Background(), q.String(), nextFunc)

		if pdebug.Enabled {
			pdebug.Printf(context.TODO(), "delayed query executed")
		}

		p.queryExecMutex.Lock()
		defer p.queryExecMutex.Unlock()

		p.queryExecTimer = nil
	})
	return true
}

func (p *Peco) PrintResults() {
	if pdebug.Enabled {
		g := pdebug.Marker(context.TODO(), "Peco.PrintResults")
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
			p.ResultCh() <- it.(line.Line)
			return true
		})
	}()

	var buf bytes.Buffer

	if pdebug.Enabled {
		pdebug.Printf(context.TODO(), "--print-query was %t", p.printQuery)
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
