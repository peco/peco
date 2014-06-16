package peco

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"
)

// Ctx contains all the important data. while you can easily access
// data in this struct from anwyehre, only do so via channels
type Ctx struct {
	result         string
	loopCh         chan struct{}
	queryCh        chan string
	drawCh         chan []Match
	pagingCh       chan PagingRequest
	mutex          sync.Mutex
	query          []rune
	caretPos       int
	selectedLine   int
	lines          []Match
	current        []Match
	config         *Config
	Matchers       []Matcher
	CurrentMatcher int
	ExitStatus     int

	wait *sync.WaitGroup
}

type Match struct {
	line    string
	matches [][]int
}

type Matcher interface {
	Match(string, []Match) []Match
	String() string
}

func NewCtx() *Ctx {
	return &Ctx{
		"",
		make(chan struct{}),         // loopCh. You never send messages to this. no point in buffering
		make(chan string, 5),        // queryCh.
		make(chan []Match, 5),       // drawCh.
		make(chan PagingRequest, 5), // pagingCh
		sync.Mutex{},
		[]rune{},
		0,
		1,
		[]Match{},
		nil,
		NewConfig(),
		[]Matcher{
			NewIgnoreCaseMatcher(),
			NewCaseSensitiveMatcher(),
			NewRegexpMatcher(),
		},
		0,
		0,
		&sync.WaitGroup{},
	}
}

func (c *Ctx) ReadConfig(file string) error {
	err := c.config.ReadFilename(file)
	if err == nil {
		c.SetCurrentMatcher(c.config.Matcher)
	}
	return err
}

func (c *Ctx) Result() string {
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

func (c *Ctx) LoopCh() chan struct{} {
	return c.loopCh
}

func (c *Ctx) QueryCh() chan string {
	return c.queryCh
}

func (c *Ctx) DrawCh() chan []Match {
	return c.drawCh
}

func (c *Ctx) PagingCh() chan PagingRequest {
	return c.pagingCh
}

func (c *Ctx) Terminate() {
	close(c.loopCh)
}

func (c *Ctx) ExecQuery(v string) {
	c.queryCh <- v
}

func (c *Ctx) DrawMatches(m []Match) {
	c.drawCh <- m
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

func (c *Ctx) ReadBuffer(input io.Reader) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		c.lines = append(c.lines, Match{line, nil})
	}

	if len(c.lines) > 0 {
		return nil
	}
	return fmt.Errorf("No buffer to work with was available")
}

func (c *Ctx) NewView() *View {
	return &View{c}
}

func (c *Ctx) NewFilter() *Filter {
	return &Filter{c}
}

func (c *Ctx) NewInput() *Input {
	return &Input{c}
}

func (c *Ctx) Finish() {
	close(c.LoopCh())
}

func (c *Ctx) SetQuery(q []rune) {
	c.query = q
	c.caretPos = len(q)
}

func (c *Ctx) Matcher() Matcher {
	return c.Matchers[c.CurrentMatcher]
}

func (c *Ctx) AddMatcher(m Matcher) {
	c.Matchers = append(c.Matchers, m)
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

func (c *Ctx) SignalHandlerLoop(sigCh chan os.Signal) {
	defer c.ReleaseWaitGroup()

	for {
		select {
		case <-c.LoopCh():
			return
		case <-sigCh:
			// XXX For future reference: DO NOT, and I mean DO NOT call
			// termbox.Close() here. Calling termbox.Close() twice in our
			// context actually BLOCKS. Can you believe it? IT BLOCKS.
			//
			// So if in main(), defer termbox.Close() blocks if we also
			// call termbox.Close() here. Not cool.
			c.Finish()
			return
		}
	}
}
