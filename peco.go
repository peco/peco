package peco

import (
	"io"
	"os"
	"reflect"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/net/context"

	"github.com/google/btree"
	"github.com/lestrrat/go-pdebug"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/pipeline"
	"github.com/peco/peco/sig"
	"github.com/pkg/errors"
)

const version = "v0.4.3"

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
	return errIgnorable{err: err}
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

func (ig *idgen) next() uint64 {
	return <-ig.ch
}

var idGenerator = newIDGen()

func New() *Peco {
	return &Peco{
		Argv:              os.Args,
		Stderr:            os.Stderr,
		Stdin:             os.Stdin,
		Stdout:            os.Stdout,
		currentLineBuffer: NewMemoryBuffer(), // XXX revisit this
		idgen:             newIDGen(),
		queryExecDelay:    50 * time.Millisecond,
		readyCh:           make(chan struct{}),
		screen:            &Termbox{},
		selection:         NewSelection(),
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

func (p *Peco) Prompt() string {
	return p.prompt
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

func (p *Peco) ResultCh() chan Line {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.resultCh
}

func (p *Peco) SetResultCh(ch chan Line) {
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
	go p.Hub().SendDraw(&DrawOptions{DisableCache: true})
}

func (p *Peco) SingleKeyJumpIndex(ch rune) (uint, bool) {
	n, ok := p.singleKeyJumpPrefixMap[ch]
	return n, ok
}

func (p *Peco) Source() pipeline.Source {
	return p.source
}

func (p *Peco) Filters() *FilterSet {
	return &p.filters
}

func (p *Peco) Query() *Query {
	return &p.query
}

func (p *Peco) QueryExecDelay() time.Duration {
	return p.queryExecDelay
}

func (p *Peco) Caret() *Caret {
	return &p.caret
}

func (p *Peco) Hub() *hub.Hub {
	return p.hub
}

func (p *Peco) Err() error {
	return p.err
}

func (p *Peco) Exit(err error) {
	p.err = err
	if cf := p.cancelFunc; cf != nil {
		cf()
	}
}

func (p *Peco) Keymap() Keymap {
	return p.keymap
}

func (p *Peco) Setup() (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.Setup").BindError(&err)
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

func (p *Peco) Run(ctx context.Context) (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.Run").BindError(&err)
		defer g.End()
	}

	// do this only once
	var readyOnce sync.Once
	defer readyOnce.Do(func() { close(p.readyCh) })

	if err := p.Setup(); err != nil {
		return errors.Wrap(err, "failed to setup peco")
	}
	// screen.Init must be called within Run() because we
	// want to make sure to call screen.Close() after getting
	// out of Run()
	p.screen.Init()
	defer p.screen.Close()

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

	loopers := []interface {
		Loop(ctx context.Context, cancel func()) error
	}{
		NewInput(p, p.Keymap(), p.screen.PollEvent()),
		NewView(p),
		NewFilter(p),
		sig.New(sig.SigReceivedHandlerFunc(func(sig os.Signal) {
			p.Exit(errors.New("received signal: " + sig.String()))
		})),
	}

	for _, l := range loopers {
		go l.Loop(ctx, cancel)
	}

	// SetupSource is done AFTER other components are ready, otherwise
	// we can't draw onto the screen while we are reading a really big
	// buffer.
	// Setup source buffer
	src, err := p.SetupSource()
	if err != nil {
		return errors.Wrap(err, "failed to setup input source")
	}
	p.source = src
	p.ResetCurrentLineBuffer()

	if pdebug.Enabled {
		pdebug.Printf("peco is now ready, go go go!")
	}

	// If this is enabled, we need to check if we have 1 line only
	// in the buffer. If we do, we select that line and bail out
	if p.selectOneAndExit {
		go func() {
			// Wait till source has read all lines
			<-p.source.SetupDone()
			// If we have only one line, we just want to bail out
			// printing that one line as the result
			if b := p.CurrentLineBuffer(); b.Size() == 1 {
				if l, err := b.LineAt(0); err == nil {
					p.resultCh = make(chan Line)
					p.Exit(nil)
					p.resultCh <- l
					close(p.resultCh)
				}
			}
		}()
	}

	readyOnce.Do(func() { close(p.readyCh) })

	// This has tobe AFTER close(p.readyCh), otherwise the query is
	// ignored by us (queries are not run until peco thinks it's ready)
	if q := p.initialQuery; q != "" {
		p.Query().Set(q)
		p.Caret().SetPos(utf8.RuneCountInString(q))
		p.ExecQuery()
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
		p.Stdout.Write(opts.help())
		return makeIgnorable(errors.New("user asked to show help message"))
	}

	if opts.OptVersion {
		p.Stdout.Write([]byte("peco version " + version + "\n"))
		return makeIgnorable(errors.New("user asked to show version"))
	}

	if opts.OptRcfile == "" {
		if file, err := LocateRcfile(locateRcfileIn); err == nil {
			opts.OptRcfile = file
		}
	}

	if opts.OptTTY != "" {
		p.Stderr.Write([]byte("Warning: --tty was never supported, and it will be removed in 0.5.x\n"))
		time.Sleep(500 * time.Millisecond) // Wait, so that the user can see it
	}

	*args = remaining

	return nil
}

func (p *Peco) SetupSource() (s *Source, err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.SetupSource").BindError(&err)
		defer g.End()
	}

	var in io.Reader
	switch {
	case len(p.args) > 1:
		f, err := os.Open(p.args[1])
		if err != nil {
			return nil, errors.Wrap(err, "failed to open file for input")
		}
		in = f
	case !util.IsTty(p.Stdin):
		in = p.Stdin
	default:
		return nil, errors.New("you must supply something to work with via filename or stdin")
	}

	src := NewSource(in, p.idgen, p.bufferSize, p.enableSep)

	// Block until we receive something from `in`
	if pdebug.Enabled {
		pdebug.Printf("Blocking until we read something in source...")
	}
	go src.Setup(p)
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

func (p *Peco) populateCommandList() error {
	for _, v := range p.config.Command {
		if len(v.Args) == 0 {
			continue
		}
		makeCommandAction(p, &v).Register("ExecuteCommand." + v.Name)
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

	p.bufferSize = opts.OptBufferSize
	p.selectOneAndExit = opts.OptSelect1
	p.initialQuery = opts.OptQuery
	p.initialFilter = opts.OptInitialFilter
	if len(p.initialFilter) <= 0 {
		p.initialFilter = p.config.InitialFilter
	}
	if len(p.initialFilter) <= 0 {
		p.initialFilter = opts.OptInitialMatcher
	}

	if err := p.populateCommandList(); err != nil {
		return errors.Wrap(err, "failed to populate command list")
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
	p.filters.Add(NewIgnoreCaseFilter())
	p.filters.Add(NewCaseSensitiveFilter())
	p.filters.Add(NewSmartCaseFilter())
	p.filters.Add(NewRegexpFilter())

	for name, c := range p.config.CustomFilter {
		f := NewExternalCmdFilter(name, c.Cmd, c.Args, c.BufferThreshold, p.idgen, p.enableSep)
		p.filters.Add(f)
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

func (p *Peco) CurrentLineBuffer() Buffer {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.currentLineBuffer
}

func (p *Peco) SetCurrentLineBuffer(b Buffer) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.SetCurrentLineBuffer %s", reflect.TypeOf(b).String())
		defer g.End()
	}
	p.currentLineBuffer = b
	go p.Hub().SendDraw(nil)
}

func (p *Peco) ResetCurrentLineBuffer() {
	p.SetCurrentLineBuffer(p.source)
}

func (p *Peco) ExecQuery() bool {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.ExecQuery")
		defer g.End()
	}

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
		p.ResetCurrentLineBuffer()
		p.Hub().SendDraw(&DrawOptions{DisableCache: true})
		return true
	}

	delay := p.QueryExecDelay()
	if delay <= 0 {
		// No delay, execute immediately
		p.Hub().SendQuery(q.String())
		return true
	}

	p.queryExecMutex.Lock()
	defer p.queryExecMutex.Unlock()

	if p.queryExecTimer != nil {
		return true
	}

	// Wait $delay millisecs before sending the query
	// if a new input comes in, batch them up
	p.queryExecTimer = time.AfterFunc(delay, func() {
		p.Hub().SendQuery(q.String())

		p.queryExecMutex.Lock()
		defer p.queryExecMutex.Unlock()

		p.queryExecTimer = nil
	})
	return true
}

func (p *Peco) CollectResults() {
	// In rare cases where the result channel is not setup
	// prior to call to this method, bail out
	if p.resultCh == nil {
		return
	}

	p.selection.Ascend(func(it btree.Item) bool {
		p.resultCh <- it.(Line)
		return true
	})
	close(p.resultCh)
}
