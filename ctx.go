package peco

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

const debug = false
const DefaultCustomFilterBufferThreshold = 100

var screen Screen = Termbox{}

// CtxOptions is the interface that defines that options can be
// passed in from the command line
type CtxOptions interface {
	// EnableNullSep should return if the null separator is
	// enabled (--null)
	EnableNullSep() bool

	// BufferSize should return the buffer size. By default (i.e.
	// when it returns 0), the buffer size is unlimited.
	// (--buffer-size)
	BufferSize() int

	// InitialIndex is the line number to put the cursor on
	// when peco starts
	InitialIndex() int

	// LayoutType returns the name of the layout to use
	LayoutType() string
}

type PageInfo struct {
	index   int
	offset  int
	perPage int
	total   int
	maxPage int
}

func (p *Ctx) CaretPos() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.caretPosition
}

func (p *Ctx) SetCaretPos(where int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.caretPosition = where
}

func (p *Ctx) MoveCaretPos(offset int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.caretPosition = p.caretPosition + offset
}

type FilterQuery struct {
	query      []rune
	savedQuery []rune
	mutex      sync.Locker
}

func (q FilterQuery) Query() []rune {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.query[:]
}

func (q FilterQuery) SavedQuery() []rune {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.savedQuery[:]
}

func (q FilterQuery) QueryString() string {
	qbytes := q.Query()
	return string(qbytes)
}

func (q FilterQuery) QueryLen() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return len(q.query)
}

func (q *FilterQuery) AppendQuery(r rune) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.query = append(q.query, r)
}

func (q *FilterQuery) InsertQueryAt(ch rune, where int) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	sq := q.query
	buf := make([]rune, len(sq)+1)
	copy(buf, sq[:where])
	buf[where] = ch
	copy(buf[where+1:], sq[where:])
	q.query = buf
}

// Ctx contains all the important data. while you can easily access
// data in this struct from anwyehre, only do so via channels
type Ctx struct {
	*Hub
	*FilterQuery
	filters             FilterSet
	caretPosition       int
	enableSep           bool
	resultCh            chan Line
	mutex               sync.Locker
	currentLine         int
	currentPage         *PageInfo
	selection           *Selection
	activeLineBuffer    LineBuffer
	rawLineBuffer       *RawLineBuffer
	lines               []Line
	linesMutex          sync.Locker
	current             []Line
	currentMutex        sync.Locker
	bufferSize          int
	config              *Config
	selectionRangeStart int
	layoutType          string

	wait *sync.WaitGroup
	err  error
}

func newMutex() sync.Locker {
	if debug {
		return &loggingMutex{&sync.Mutex{}}
	}
	return &sync.Mutex{}
}

type loggingMutex struct {
	*sync.Mutex
}

func (m *loggingMutex) Lock() {
	buf := make([]byte, 8092)
	l := runtime.Stack(buf, false)
	fmt.Printf("LOCK %s\n", buf[:l])
	m.Mutex.Lock()
}

func (m *loggingMutex) Unlock() {
	buf := make([]byte, 8092)
	l := runtime.Stack(buf, false)
	fmt.Printf("UNLOCK %s\n", buf[:l])
	m.Mutex.Unlock()
}

func NewCtx(o CtxOptions) *Ctx {
	return newCtx(o, 5)
}

func newCtx(o CtxOptions, hubBufferSize int) *Ctx {
	c := &Ctx{
		Hub:                 NewHub(hubBufferSize),
		FilterQuery:         &FilterQuery{[]rune{}, []rune{}, newMutex()},
		filters:             FilterSet{},
		caretPosition:       0,
		resultCh:            nil,
		mutex:               newMutex(),
		currentPage:         &PageInfo{0, 1, 0, 0, 0},
		selection:           NewSelection(),
		activeLineBuffer:    nil,
		rawLineBuffer:       NewRawLineBuffer(),
		lines:               []Line{},
		linesMutex:          newMutex(),
		current:             nil,
		currentMutex:        newMutex(),
		config:              NewConfig(),
		selectionRangeStart: invalidSelectionRange,
		wait:                &sync.WaitGroup{},
		layoutType:          "top-down",
	}

	if o != nil {
		// XXX Pray this is really nil :)
		c.enableSep = o.EnableNullSep()
		c.currentLine = o.InitialIndex()

		c.rawLineBuffer.SetCapacity(o.BufferSize())

		if v := o.LayoutType(); v != "" {
			c.layoutType = v
		}
	}

	c.filters.Add(NewIgnoreCaseFilter())
	c.filters.Add(NewCaseSensitiveFilter())
	c.filters.Add(NewSmartCaseFilter())

	return c
}

const invalidSelectionRange = -1

func (c *Ctx) ReadConfig(file string) error {
	if err := c.config.ReadFilename(file); err != nil {
		return err
	}

	if err := c.LoadCustomFilter(); err != nil {
		return err
	}

	c.SetCurrentFilterByName(c.config.InitialFilter)

	if c.layoutType == "" { // Not set yet
		if c.config.Layout != "" {
			c.layoutType = c.config.Layout
		}
	}

	return nil
}

func (c *Ctx) SetLines(newLines []Line) {
	c.linesMutex.Lock()
	defer c.linesMutex.Unlock()
	c.lines = newLines
}

func (c *Ctx) GetLines() []Line {
	c.linesMutex.Lock()
	defer c.linesMutex.Unlock()
	return c.lines[:]
}

func (c *Ctx) GetLinesCount() int {
	c.linesMutex.Lock()
	defer c.linesMutex.Unlock()
	return len(c.lines)
}

func (c *Ctx) IsBufferOverflowing() bool {
	if c.bufferSize <= 0 {
		return false
	}

	return len(c.lines) > c.bufferSize
}

func (c *Ctx) IsRangeMode() bool {
	return c.selectionRangeStart != invalidSelectionRange
}

func (c *Ctx) SelectionLen() int {
	return c.selection.Len()
}

func (c *Ctx) SelectionAdd(x int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if l, err := c.GetCurrentLineBuffer().LineAt(x); err == nil {
		c.selection.Add(l)
	}
}

func (c *Ctx) SelectionRemove(x int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if l, err := c.GetCurrentLineBuffer().LineAt(x); err == nil {
		c.selection.Delete(l)
	}
}

func (c *Ctx) SelectionClear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.selection = NewSelection()
}

func (c *Ctx) SelectionContains(n int) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if l, err := c.GetCurrentLineBuffer().LineAt(n); err == nil {
		return c.selection.Has(l)
	}
	return false
}

func (c *Ctx) GetCurrent() []Line {
	c.currentMutex.Lock()
	defer c.currentMutex.Unlock()
	return c.current[:]
}

func (c *Ctx) GetCurrentLen() int {
	c.currentMutex.Lock()
	defer c.currentMutex.Unlock()
	return len(c.current)
}

func (c *Ctx) GetCurrentAt(i int) Line {
	c.currentMutex.Lock()
	defer c.currentMutex.Unlock()

	if i < 0 || len(c.current) <= i {
		panic(fmt.Sprintf("GetCurrentAt: index out of range (%d)", i))
	}
	return c.current[i]
}

func (c *Ctx) ResultCh() <-chan Line {
	return c.resultCh
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
	tracer.Printf("Ctx.ExecQuery: START")
	defer tracer.Printf("Ctx.ExecQuery: END")

	if c.QueryLen() <= 0 {
		tracer.Printf("Ctx.ExecQuery: Nothing to do")
		return false
	}

	tracer.Printf("Ctx.ExecQuery: SendQuery")
	c.SendQuery(c.QueryString())
	return true
}

func (c *Ctx) DrawPrompt() {
	c.SendDrawPrompt()
}

func (c *Ctx) Buffer() []Line {
	// Copy lines so it's safe to read it
	lcopy := make([]Line, len(c.lines))
	copy(lcopy, c.lines)
	return lcopy
}

func (c *Ctx) NewBufferReader(r io.ReadCloser) *BufferReader {
	return &BufferReader{c, r, make(chan struct{})}
}

func (c *Ctx) NewView() *View {
	var layout Layout
	switch c.layoutType {
	case "bottom-up":
		layout = NewBottomUpLayout(c)
	default:
		layout = NewDefaultLayout(c)
	}
	return &View{c, newMutex(), layout}
}

func (c *Ctx) NewFilter() *Filter {
	return &Filter{c}
}

func (c *Ctx) NewInput() *Input {
	// Create a new keymap object
	k := NewKeymap(c.config.Keymap, c.config.Action)
	k.ApplyKeybinding()
	return &Input{c, newMutex(), nil, k, []string{}}
}

func (c *Ctx) SetSavedQuery(q []rune) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.FilterQuery.savedQuery = q
}

func (c *Ctx) SetQuery(q []rune) {
	tracer.Printf("Ctx.SetQuery: START")
	defer tracer.Printf("Ctx.SetQuery: END")
	c.mutex.Lock()
	tracer.Printf("Ctx.SetQuery: setting query to '%s'", string(q))
	c.FilterQuery.query = q
	c.mutex.Unlock()
	c.SetCaretPos(c.QueryLen())
}

func (c *Ctx) Filter() QueryFilterer {
	return c.filters.GetCurrent()
}

func (c *Ctx) LoadCustomFilter() error {
	if len(c.config.CustomFilter) == 0 {
		return nil
	}

	for name, cfg := range c.config.CustomFilter {
		f := NewExternalCmdFilter(name, cfg.Cmd, cfg.Args, cfg.BufferThreshold, c.enableSep)
		if err := c.filters.Add(f); err != nil {
			return err
		}
	}
	return nil
}

type signalHandler struct {
	*Ctx
	sigCh chan os.Signal
}

func (c *Ctx) NewSignalHandler() *signalHandler {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	return &signalHandler{c, sigCh}
}

func (s *signalHandler) Loop() {
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
			s.ExitWith(fmt.Errorf("received signal"))
			return
		}
	}
}

func (c *Ctx) Error() error {
	return c.err
}

func (c *Ctx) ExitWith(err error) {
	c.err = err
	c.Stop()
}

func (c *Ctx) SetPrompt(p string) {
	c.config.Prompt = p
}

func (c *Ctx) AddRawLine(l *RawLine) {
	c.rawLineBuffer.AppendLine(l)
}

func (c Ctx) GetRawLineBufferSize() int {
	return c.rawLineBuffer.Size()
}

func (c *Ctx) ResetActiveLineBuffer() {
	c.rawLineBuffer.Replay()
	c.SetActiveLineBuffer(c.rawLineBuffer)
}

func (c *Ctx) SetActiveLineBuffer(l *RawLineBuffer) {
	c.activeLineBuffer = l

	go func(l *RawLineBuffer) {
		prev := time.Time{}
		for _ = range l.OutputCh() {
			if time.Since(prev) > time.Millisecond {
				c.SendDraw()
				prev = time.Now()
			}
		}
		c.SendDraw()
	}(l)
}

func (c Ctx) GetCurrentLineBuffer() LineBuffer {
	if b := c.activeLineBuffer; b != nil {
		return b
	}
	return c.rawLineBuffer
}

func (c *Ctx) RotateFilter() {
	c.filters.Rotate()
}

func (c *Ctx) SetCurrentFilterByName(name string) error {
	return c.filters.SetCurrentByName(name)
}

func (c *Ctx) startInput() {
	c.AddWaitGroup(1)
	go c.NewInput().Loop()
}
