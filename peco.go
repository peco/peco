package peco

import (
	"os"
	"reflect"
	"time"

	"golang.org/x/net/context"

	"github.com/google/btree"
	"github.com/lestrrat/go-pdebug"
	"github.com/peco/peco/hub"
	"github.com/peco/peco/internal/util"
	"github.com/peco/peco/pipeline"
	"github.com/peco/peco/sig"
	"github.com/pkg/errors"
)

const version = "v0.3.6"

type errIgnorable struct {
	err error
}

func (e errIgnorable) Ignorable() bool { return true }
func (e errIgnorable) Causer() error {
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

func New() *Peco {
	return &Peco{
		Argv:              os.Args,
		currentLineBuffer: NewMemoryBuffer(), // XXX revisit this
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

func (p *Peco) Context() context.Context {
	return p.ctx
}

func (p *Peco) LayoutType() string {
	return p.layoutType
}

func (p *Peco) Location() *Location {
	return &p.location
}

func (p *Peco) ResultCh() chan Line {
	return p.resultCh
}

func (p *Peco) SetResultCh(ch chan Line) {
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
}

func (p *Peco) SingleKeyJumpIndex(ch rune) (uint, bool) {
	// FIXME: use p.keyjump or something instead of p.config
	n, ok := p.config.SingleKeyJump.PrefixMap[ch]
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

	if err := parseCommandLine(&p.Options, &p.args, p.Argv); err != nil {
		return errors.Wrap(err, "failed to parse command line")
	}

	// Read config
	if err := readConfig(&p.config, p.Options.OptRcfile); err != nil {
		return errors.Wrap(err, "failed to setup configuration")
	}

	// Take Args, Config, Options, and apply the configuration to
	// the peco object
	if err := p.ApplyConfig(); err != nil {
		return errors.Wrap(err, "failed to apply configuration")
	}

	// XXX p.Keymap et al should be initialized around here
	p.hub = hub.New(5)

	// Setup source buffer
	src, err := p.SetupSource()
	if err != nil {
		return errors.Wrap(err, "failed to setup input source")
	}
	p.source = src
	p.ResetCurrentLineBuffer()
	return nil
}

func (p *Peco) Run(ctx context.Context) (err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.Run").BindError(&err)
		defer g.End()
	}

	if err := p.Setup(); err != nil {
		return errors.Wrap(err, "failed to setup peco")
	}
	// screen.Init must be called within Run() because we
	// want to make sure to call screen.Close() after getting
	// out of Run()
	p.screen.Init()
	defer p.screen.Close()

	var _cancel func()
	ctx, _cancel = context.WithCancel(ctx)
	cancel := func() {
		if pdebug.Enabled {
			pdebug.Printf("Peco.Run cancel called")
		}
		_cancel()
	}

	// keep *this* ctx (not the Background one), as calling `cancel`
	// only affects the wrapped context
	p.ctx = ctx

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

	if pdebug.Enabled {
		pdebug.Printf("peco is now ready, go go go!")
	}

	close(p.readyCh)
	<-ctx.Done()

	return p.Err()
}

func parseCommandLine(opts *CLIOptions, args *[]string, argv []string) error {
	remaining, err := opts.parse(argv)
	if err != nil {
		return errors.Wrap(err, "failed to parse command line options")
	}

	if opts.OptHelp {
		os.Stdout.Write(opts.help())
		return makeIgnorable(errors.New("user asked to show help message"))
	}

	if opts.OptRcfile == "" {
		if file, err := LocateRcfile(locateRcfileIn); err != nil {
			opts.OptRcfile = file
		}
	}

	*args = remaining

	return nil
}

func (p *Peco) SetupSource() (s *Source, err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.SetupSource").BindError(&err)
		defer g.End()
	}

	var in *os.File
	switch {
	case len(p.args) > 1:
		in, err = os.Open(p.args[1])
		if err != nil {
			return nil, errors.Wrap(err, "failed to open file for input")
		}
	case !util.IsTty(os.Stdin.Fd()):
		in = os.Stdin
	default:
		return nil, errors.New("you must supply something to work with via filename or stdin")
	}
	defer in.Close()

	src := NewSource(in, p.enableSep)

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
		makeCommandAction(&v).Register("ExecuteCommand." + v.Name)
	}

	return nil
}

func (p *Peco) ApplyConfig() error {
	// If layoutType is not set and is set in the config, set it
	if p.layoutType == "" {
		if v := p.config.Layout; v != "" {
			p.layoutType = v
		} else {
			p.layoutType = DefaultLayoutType
		}
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

	return nil
}

func (p *Peco) populateFilters() error {
	p.filters.Add(NewIgnoreCaseFilter())
	p.filters.Add(NewCaseSensitiveFilter())
	p.filters.Add(NewSmartCaseFilter())
	p.filters.Add(NewRegexpFilter())
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
	p.styles = *(p.config.Style)
	return nil
}

func (p *Peco) CurrentLineBuffer() Buffer {
	return p.currentLineBuffer
}

func (p *Peco) SetCurrentLineBuffer(b Buffer) {
	if pdebug.Enabled {
		g := pdebug.Marker("Peco.SetCurrentLineBuffer %s", reflect.TypeOf(b).String())
		defer g.End()
	}
	p.currentLineBuffer = b
	p.Hub().SendDraw(false)
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

func (p *Peco) collectResults() {
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
