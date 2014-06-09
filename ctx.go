package percol

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/nsf/termbox-go"
)

// Ctx contains all the important data. while you can easily access
// data in this struct from anwyehre, only do so via channels
type Ctx struct {
	result       string
	loopCh       chan struct{}
	queryCh      chan string
	drawCh       chan []Match
	mutex        sync.Mutex
	query        []rune
	selectedLine int
	lines        []Match
	current      []Match

	wait *sync.WaitGroup
}

type Match struct {
	line    string
	matches [][]int
}

func NewCtx() *Ctx {
	return &Ctx{
		"",
		make(chan struct{}), // loopCh
		make(chan string),   // queryCh
		make(chan []Match),  // drawCh
		sync.Mutex{},
		[]rune{},
		1,
		[]Match{},
		nil,
		&sync.WaitGroup{},
	}
}

func (c *Ctx) Result() string {
	return c.result
}

func (c *Ctx) AddWaitGroup() {
	c.wait.Add(1)
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

func (c *Ctx) ReadBuffer(input io.Reader) {
	rdr := bufio.NewReader(input)
	for {
		line, err := rdr.ReadString('\n')
		if err != nil {
			break
		}
		c.lines = append(c.lines, Match{line, nil})
	}
}

func (c *Ctx) PrintResult() {
	if r := c.result; r != "" {
		fmt.Fprintln(os.Stderr, c.result)
	}
}

func (c *Ctx) NewUI() *UI {
	return &UI{c}
}

func (c *Ctx) NewFilter() *Filter {
	return &Filter{c}
}

func (c *Ctx) Loop() {
	c.AddWaitGroup()
	defer c.ReleaseWaitGroup()

	for {
		select {
		case <-c.LoopCh(): // can only fall here if we closed c.loopCh
			return
		default:
			ev := termbox.PollEvent()
			if ev.Type == termbox.EventError {
				//update = false
			} else if ev.Type == termbox.EventKey {
				c.handleKeyEvent(ev)
			}
		}
	}
}

func (c *Ctx) handleKeyEvent(ev termbox.Event) {
	switch ev.Key {
	case termbox.KeyEsc:
		close(c.LoopCh())
	case termbox.KeyEnter:
		if len(c.current) == 1 {
			c.result = c.current[0].line
		} else if c.selectedLine > 0 && c.selectedLine < len(c.current) {
			c.result = c.current[c.selectedLine].line
		}
		close(c.LoopCh())
	case termbox.KeyArrowUp, termbox.KeyCtrlK:
		c.selectedLine--
		c.DrawMatches(nil)
	case termbox.KeyArrowDown, termbox.KeyCtrlJ:
		c.selectedLine++
		c.DrawMatches(nil)
	case termbox.KeyBackspace, termbox.KeyBackspace2:
		if len(c.query) > 0 {
			c.query = c.query[:len(c.query)-1]
			if len(c.query) > 0 {
				c.ExecQuery(string(c.query))
			} else {
				c.current = nil
				c.DrawMatches(nil)
			}
		}
	default:
		if ev.Key == termbox.KeySpace {
			ev.Ch = ' '
		}

		if ev.Ch > 0 {
			c.query = append(c.query, ev.Ch)
			c.ExecQuery(string(c.query))
		}
	}
}
