package peco

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type CtxOptions interface {
	EnableNullSep() bool
	BufferSize() int
	InitialIndex() int
}

type PageInfo struct {
	index int
	offset int
	perPage int
}

// Ctx contains all the important data. while you can easily access
// data in this struct from anwyehre, only do so via channels
type Ctx struct {
	*Hub
	enableSep           bool
	result              []Match
	mutex               sync.Mutex
	query               []rune
	prompt              []rune
	caretPos            int
	currentLine         int
	currentPage         PageInfo
	selection           Selection
	lines               []Match
	current             []Match
	bufferSize          int
	config              *Config
	Matchers            []Matcher
	CurrentMatcher      int
	ExitStatus          int
	selectionRangeStart int

	wait *sync.WaitGroup
}

func NewCtx(o CtxOptions) *Ctx {
	return &Ctx{
		NewHub(),
		o.EnableNullSep(),
		[]Match{},
		sync.Mutex{},
		[]rune{},
		[]rune{},
		0,
		o.InitialIndex(),
		struct{ index, offset, perPage int }{0, 1, 0},
		Selection([]int{}),
		[]Match{},
		nil,
		o.BufferSize(),
		NewConfig(),
		[]Matcher{
			NewIgnoreCaseMatcher(o.EnableNullSep()),
			NewCaseSensitiveMatcher(o.EnableNullSep()),
			NewRegexpMatcher(o.EnableNullSep()),
		},
		0,
		0,
		NoSelectionRange,
		&sync.WaitGroup{},
	}
}

const NoSelectionRange = -1

func (c *Ctx) ReadConfig(file string) error {
	if err := c.config.ReadFilename(file); err != nil {
		return err
	}

	if err := c.LoadCustomMatcher(); err != nil {
		return err
	}
	c.SetCurrentMatcher(c.config.Matcher)

	return nil
}

func (c *Ctx) IsBufferOverflowing() bool {
	if c.bufferSize <= 0 {
		return false
	}

	return len(c.lines) > c.bufferSize
}

func (c *Ctx) IsRangeMode() bool {
	return c.selectionRangeStart != NoSelectionRange
}

func (c *Ctx) SelectedRange() Selection {
	if !c.IsRangeMode() {
		return Selection{}
	}

	selectedLines := []int{}
	if c.selectionRangeStart < c.currentLine {
		for i := c.selectionRangeStart; i < c.currentLine; i++ {
			selectedLines = append(selectedLines, i)
		}
	} else {
		for i := c.selectionRangeStart; i > c.currentLine; i-- {
			selectedLines = append(selectedLines, i)
		}
	}
	return Selection(selectedLines)
}

func (c *Ctx) Result() []Match {
	return c.result
}

func (c *Ctx) AddWaitGroup(v int) {
	c.wait.Add(v)
}

func (c *Ctx) ReleaseWaitGroup() {
	c.wait.Done()
}

func (c *Ctx) WaitDone() {
	c.wait.Wait()
}

func (c *Ctx) ExecQuery() bool {
	if len(c.query) > 0 {
		c.QueryCh() <- string(c.query)
		return true
	}
	return false
}

func (c *Ctx) DrawMatches(m []Match) {
	c.DrawCh() <- m
}
func (c *Ctx) Refresh() {
	c.DrawMatches(nil)
}

func (c *Ctx) Buffer() []Match {
	// Copy lines so it's safe to read it
	lcopy := make([]Match, len(c.lines))
	copy(lcopy, c.lines)
	return lcopy
}

func (c *Ctx) NewBufferReader(r io.ReadCloser) *BufferReader {
	return &BufferReader{c, r, make(chan struct{})}
}

func (c *Ctx) NewView() *View {
	return &View{c}
}

func (c *Ctx) NewFilter() *Filter {
	return &Filter{c, make(chan string)}
}

func (c *Ctx) NewInput() *Input {
	// Create a new keymap object
	k := NewKeymap(c.config.Keymap, c.config.Action)
	k.ApplyKeybinding()
	return &Input{c, &sync.Mutex{}, nil, k}
}

func (c *Ctx) SetQuery(q []rune) {
	c.query = q
	c.caretPos = len(q)
}

func (c *Ctx) Matcher() Matcher {
	return c.Matchers[c.CurrentMatcher]
}

func (c *Ctx) AddMatcher(m Matcher) error {
	if err := m.Verify(); err != nil {
		return fmt.Errorf("Verification for custom matcher failed: %s", err)
	}
	c.Matchers = append(c.Matchers, m)
	return nil
}

func (c *Ctx) SetCurrentMatcher(n string) bool {
	for i, m := range c.Matchers {
		if m.String() == n {
			c.CurrentMatcher = i
			return true
		}
	}
	return false
}

func (c *Ctx) LoadCustomMatcher() error {
	if len(c.config.CustomMatcher) == 0 {
		return nil
	}

	for name, args := range c.config.CustomMatcher {
		if err := c.AddMatcher(NewCustomMatcher(c.enableSep, name, args)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Ctx) ExitWith(i int) {
	c.ExitStatus = i
	c.Stop()
}

type SignalHandler struct {
	*Ctx
	sigCh chan os.Signal
}

func (c *Ctx) NewSignalHandler() *SignalHandler {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return &SignalHandler{c, sigCh}
}

func (s *SignalHandler) Loop() {
	defer s.ReleaseWaitGroup()

	for {
		select {
		case <-s.LoopCh():
			return
		case <-s.sigCh:
			// XXX For future reference: DO NOT, and I mean DO NOT call
			// termbox.Close() here. Calling termbox.Close() twice in our
			// context actually BLOCKS. Can you believe it? IT BLOCKS.
			//
			// So if we called termbox.Close() here, and then in main()
			// defer termbox.Close() blocks. Not cool.
			s.ExitWith(1)
			return
		}
	}
}

func (c *Ctx) SetPrompt(p []rune) {
	c.prompt = p
}
