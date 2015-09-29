package peco

import (
	"io"
	"sync"
	"time"
)

func (c *Ctx) CaretPos() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.caretPosition
}

func (c *Ctx) SetCaretPos(where int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.caretPosition = where
}

func (c *Ctx) MoveCaretPos(offset int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.caretPosition = c.caretPosition + offset
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
		currentCol:          0,
		currentPage:         &PageInfo{},
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
		singleKeyJumpMode:   false,
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
	c.filters.Add(NewRegexpFilter())

	jumpMap := make(map[rune]uint)
	chrs := "asdfghjklzxcvbnmqwertyuiop"
	for i := 0; i < len(chrs); i++ {
		jumpMap[rune(chrs[i])] = uint(i)
	}
	c.config.SingleKeyJumpMap = jumpMap
	c.populateSingleKeyJumpList()

	return c
}

func (c *Ctx) populateSingleKeyJumpList() {
	c.config.SingleKeyJumpList = make([]rune, len(c.config.SingleKeyJumpMap))
	for k, v := range c.config.SingleKeyJumpMap {
		c.config.SingleKeyJumpList[v] = k
	}
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

	c.populateSingleKeyJumpList()

	return nil
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

var execQueryLock = newMutex()
var execQueryTimer *time.Timer

func (c *Ctx) ExecQuery() bool {
	trace("Ctx.ExecQuery: START")
	defer trace("Ctx.ExecQuery: END")

	if c.QueryLen() <= 0 {
		if c.activeLineBuffer != nil {
			c.ResetActiveLineBuffer()
			return true
		}
		return false
	}

	delay := c.config.QueryExecutionDelay

	if delay <= 0 {
		c.SendQuery(c.QueryString())
		return true
	}

	go func() {
		// Wait $delay millisecs before sending the query
		// if a new input comes in, batch them up
		execQueryLock.Lock()
		defer execQueryLock.Unlock()
		if execQueryTimer != nil {
			return
		}
		execQueryTimer = time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
			trace("Ctx.ExecQuery: Sending Query!")
			c.SendQuery(c.QueryString())

			execQueryLock.Lock()
			defer execQueryLock.Unlock()
			execQueryTimer = nil
		})
	}()
	return true
}

func (c *Ctx) DrawPrompt() {
	c.SendDrawPrompt()
}

func (c *Ctx) NewBufferReader(r io.ReadCloser) *BufferReader {
	return &BufferReader{c, r, make(chan struct{}, 1)}
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
	trace("Ctx.SetQuery: START")
	defer trace("Ctx.SetQuery: END")
	c.mutex.Lock()
	trace("Ctx.SetQuery: setting query to '%s'", string(q))
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
	c.SetActiveLineBuffer(c.rawLineBuffer, false)
}

func (c *Ctx) SetActiveLineBuffer(l *RawLineBuffer, runningQuery bool) {
	c.activeLineBuffer = l

	go func(l *RawLineBuffer) {
		prev := time.Time{}
		for _ = range l.OutputCh() {
			if time.Since(prev) > time.Millisecond {
				c.SendDraw(runningQuery)
				prev = time.Now()
			}
		}
		c.SendDraw(runningQuery)
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

func (c *Ctx) ResetSelectedFilter() {
	c.filters.Reset()
}

func (c *Ctx) SetCurrentFilterByName(name string) error {
	return c.filters.SetCurrentByName(name)
}

func (c *Ctx) startInput() {
	c.AddWaitGroup(1)
	go c.NewInput().Loop()
}

func (c *Ctx) ToggleSingleKeyJumpMode() {
	c.singleKeyJumpMode = !c.singleKeyJumpMode
	c.SendPurgeDisplayCache()
	c.SendDraw(false)
}

func (c *Ctx) IsSingleKeyJumpMode() bool {
	return c.singleKeyJumpMode
}
