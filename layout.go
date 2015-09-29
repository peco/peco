package peco

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

var extraOffset = 0

// Crop returns a new LineBuffer whose contents are
// bound within the given range
func (pf PageCrop) Crop(in LineBuffer) LineBuffer {
	out := &FilteredLineBuffer{
		src:       in,
		selection: []int{},
	}

	s := pf.perPage * (pf.currentPage - 1)
	e := s + pf.perPage
	if s > in.Size() {
		return out
	}
	if e >= in.Size() {
		e = in.Size()
	}

	for i := s; i < e; i++ {
		out.SelectSourceLineAt(i)
	}
	return out
}

const (
	// LayoutTypeTopDown is the default. All the items read from top to bottom
	LayoutTypeTopDown = "top-down"
	// LayoutTypeBottomUp changes the layout to read from bottom to up
	LayoutTypeBottomUp = "bottom-up"
)

// IsValidLayoutType checks if a string is a supported layout type
func IsValidLayoutType(v LayoutType) bool {
	return v == LayoutTypeTopDown || v == LayoutTypeBottomUp
}

const (
	// AnchorTop anchors elements towards the top of the screen
	AnchorTop VerticalAnchor = iota + 1
	// AnchorBottom anchors elements towards the bottom of the screen
	AnchorBottom
)

// IsValidVerticalAnchor checks if the specified anchor is supported
func IsValidVerticalAnchor(anchor VerticalAnchor) bool {
	return anchor == AnchorTop || anchor == AnchorBottom
}

// Utility function
func mergeAttribute(a, b termbox.Attribute) termbox.Attribute {
	if a&0x0F == 0 || b&0x0F == 0 {
		return a | b
	}
	return ((a - 1) | (b - 1)) + 1
}

// Utility function
func printScreen(x, y int, fg, bg termbox.Attribute, msg string, fill bool) int {
	return printScreenWithOffset(x, y, 0, fg, bg, msg, fill)
}

func printScreenWithOffset(x, y, xOffset int, fg, bg termbox.Attribute, msg string, fill bool) int {
	var written int

	for len(msg) > 0 {
		c, w := utf8.DecodeRuneInString(msg)
		if c == utf8.RuneError {
			c = '?'
			w = 1
		}
		msg = msg[w:]
		if c == '\t' {
			// In case we found a tab, we draw it as 4 spaces
			n := 4 - (x+xOffset)%4
			for i := 0; i <= n; i++ {
				screen.SetCell(x+i, y, ' ', fg, bg)
			}
			written += n
			x += n
		} else {
			screen.SetCell(x, y, c, fg, bg)
			n := runewidth.RuneWidth(c)
			x += n
			written += n
		}
	}

	if !fill {
		return written
	}

	width, _ := screen.Size()
	for ; x < width; x++ {
		screen.SetCell(x, y, ' ', fg, bg)
	}
	written += width - x
	return written
}

// NewAnchorSettings creates a new AnchorSetting struct. Panics if
// an unknown VerticalAnchor is sent
func NewAnchorSettings(anchor VerticalAnchor, offset int) *AnchorSettings {
	if !IsValidVerticalAnchor(anchor) {
		panic("Invalid vertical anchor specified")
	}

	return &AnchorSettings{anchor, offset}
}

// AnchorPosition returns the starting y-offset, based on the
// anchor type and offset
func (as AnchorSettings) AnchorPosition() int {
	var pos int
	switch as.anchor {
	case AnchorTop:
		pos = as.anchorOffset
	case AnchorBottom:
		_, h := screen.Size()
		pos = h - as.anchorOffset - 1 // -1 is required because y is 0 base, but h is 1 base
	default:
		panic("Unknown anchor type!")
	}

	return pos
}

// NewUserPrompt creates a new UserPrompt struct
func NewUserPrompt(ctx *Ctx, anchor VerticalAnchor, anchorOffset int) *UserPrompt {
	prefix := ctx.config.Prompt
	if len(prefix) <= 0 { // default
		prefix = "QUERY>"
	}
	prefixLen := runewidth.StringWidth(prefix)

	return &UserPrompt{
		Ctx:            ctx,
		AnchorSettings: &AnchorSettings{anchor, anchorOffset},
		prefix:         prefix,
		prefixLen:      prefixLen,
		basicStyle:     ctx.config.Style.Basic,
		queryStyle:     ctx.config.Style.Query,
	}
}

// Draw draws the query prompt
func (u UserPrompt) Draw() {
	trace("UserPrompt.Draw: START")
	defer trace("UserPrompt.Draw: END")

	location := u.AnchorPosition()

	// print "QUERY>"
	printScreen(0, location, u.basicStyle.fg, u.basicStyle.bg, u.prefix, false)

	pos := u.CaretPos()
	if pos <= 0 { // XXX Do we really need this?
		u.SetCaretPos(0) // sanity
	}

	qs := u.QueryString()
	ql := u.QueryLen()
	if pos > ql { // XXX Do we really need this?
		u.SetCaretPos(ql)
	}

	fg := u.queryStyle.fg
	bg := u.queryStyle.bg
	switch ql {
	case 0:
		printScreen(u.prefixLen, location, fg, bg, "", true)
		printScreen(u.prefixLen+1, location, fg|termbox.AttrReverse, bg|termbox.AttrReverse, " ", false)
	case u.CaretPos():
		// the entire string + the caret after the string
		printScreen(u.prefixLen, location, fg, bg, "", true)
		printScreen(u.prefixLen+1, location, fg, bg, qs, false)
		printScreen(u.prefixLen+runewidth.StringWidth(qs)+1, location, fg|termbox.AttrReverse, bg|termbox.AttrReverse, " ", false)
	default:
		// the caret is in the middle of the string
		prev := 0
		for i, r := range []rune(u.Query()) {
			fg := u.queryStyle.fg
			bg := u.queryStyle.bg
			if i == u.CaretPos() {
				fg |= termbox.AttrReverse
				bg |= termbox.AttrReverse
			}
			screen.SetCell(u.prefixLen+1+prev, location, r, fg, bg)
			prev += runewidth.RuneWidth(r)
		}
		fg := u.queryStyle.fg
		bg := u.queryStyle.bg
		printScreen(u.prefixLen+prev+1, location, fg, bg, "", true)
	}

	width, _ := screen.Size()

	pmsg := fmt.Sprintf("%s [%d (%d/%d)]", u.Filter().String(), u.currentPage.total, u.currentPage.page, u.currentPage.maxPage)
	printScreen(width-runewidth.StringWidth(pmsg), location, u.basicStyle.fg, u.basicStyle.bg, pmsg, false)

	screen.Flush()
}

// NewStatusBar creates a new StatusBar struct
func NewStatusBar(ctx *Ctx, anchor VerticalAnchor, anchorOffset int) *StatusBar {
	return &StatusBar{
		Ctx:            ctx,
		AnchorSettings: NewAnchorSettings(anchor, anchorOffset),
		clearTimer:     nil,
		timerMutex:     newMutex(),
		basicStyle:     ctx.config.Style.Basic,
	}
}

func (s *StatusBar) stopTimer() {
	s.timerMutex.Lock()
	defer s.timerMutex.Unlock()
	if t := s.clearTimer; t != nil {
		t.Stop()
		s.clearTimer = nil
	}
}

func (s *StatusBar) setClearTimer(t *time.Timer) {
	s.timerMutex.Lock()
	defer s.timerMutex.Unlock()
	s.clearTimer = t
}

// PrintStatus prints a new status message. This also resets the
// timer created by ClearStatus()
func (s *StatusBar) PrintStatus(msg string, clearDelay time.Duration) {
	s.stopTimer()

	s.timerMutex.Lock()

	location := s.AnchorPosition()

	w, _ := screen.Size()
	width := runewidth.StringWidth(msg)
	for width > w {
		_, rw := utf8.DecodeRuneInString(msg)
		width = width - rw
		msg = msg[rw:]
	}

	var pad []byte
	if w > width {
		pad = make([]byte, w-width)
		for i := 0; i < w-width; i++ {
			pad[i] = ' '
		}
	}

	fgAttr := s.basicStyle.fg
	bgAttr := s.basicStyle.bg

	if w > width {
		printScreen(0, location, fgAttr, bgAttr, string(pad), false)
	}

	if width > 0 {
		printScreen(w-width, location, fgAttr|termbox.AttrReverse|termbox.AttrBold|termbox.AttrReverse, bgAttr|termbox.AttrReverse, msg, false)
	}
	screen.Flush()

	s.timerMutex.Unlock()

	// if everything is successful AND the clearDelay timer is specified,
	// then set a timer to clear the status
	if clearDelay != 0 {
		s.setClearTimer(time.AfterFunc(clearDelay, func() {
			s.PrintStatus("", 0)
		}))
	}
}

// NewListArea creates a new ListArea struct
func NewListArea(ctx *Ctx, anchor VerticalAnchor, anchorOffset int, sortTopDown bool) *ListArea {
	return &ListArea{
		Ctx:                 ctx,
		AnchorSettings:      NewAnchorSettings(anchor, anchorOffset),
		sortTopDown:         sortTopDown,
		displayCache:        []Line{},
		dirty:               false,
		basicStyle:          ctx.config.Style.Basic,
		queryStyle:          ctx.config.Style.Query,
		matchedStyle:        ctx.config.Style.Matched,
		selectedStyle:       ctx.config.Style.Selected,
		savedSelectionStyle: ctx.config.Style.SavedSelection,
	}
}

func (l *ListArea) purgeDisplayCache() {
	l.displayCache = []Line{}
}

func (l *ListArea) IsDirty() bool {
	return l.dirty
}

func (l *ListArea) SetDirty(dirty bool) {
	l.dirty = dirty
}

// Draw displays the ListArea on the screen
func (l *ListArea) Draw(parent Layout, perPage int, runningQuery bool) {
	trace("ListArea.Draw: START")
	defer trace("ListArea.Draw: END")
	currentPage := l.currentPage

	linebuf := l.GetCurrentLineBuffer()

	// Should only get into this clause if we are RUNNING A QUERY.
	// regular paging shouldn't be affected. This clause basically
	// makes sure that we never have an empty screen when we are
	// at a large enough page, but we don't have enough entries
	// to fill that many pages in the buffer
	if runningQuery {
		bufsiz := linebuf.Size()
		page := currentPage.page

		for page > 1 {
			if (currentPage.perPage*(page-1) < bufsiz) &&
				(currentPage.perPage*page) >= bufsiz {
				break
			}

			page--
		}
		if currentPage.page != page {
			currentPage.page = page
			parent.DrawPrompt()
		}
	}

	pf := PageCrop{perPage: currentPage.perPage, currentPage: currentPage.page}
	buf := pf.Crop(linebuf)
	bufsiz := buf.Size()

	// This protects us from losing the selected line in case our selected
	// line is greater than the buffer
	if lbufsiz := linebuf.Size(); l.currentLine >= lbufsiz {
		l.currentLine = lbufsiz - 1
	}

	// previously drawn lines are cached. first, truncate the cache
	// to current size of the drawable area
	if ldc := len(l.displayCache); ldc != perPage {
		newCache := make([]Line, perPage)
		copy(newCache, l.displayCache)
		l.displayCache = newCache
	}

	var y int
	start := l.AnchorPosition()

	// If our buffer is smaller than perPage, we may need to
	// clear some lines
	trace("ListeArea.Draw: buffer size is %d, our view area is %d\n", bufsiz, perPage)
	for n := bufsiz; n < perPage; n++ {
		l.displayCache[n] = nil
		if l.sortTopDown {
			y = n + start
		} else {
			y = start - n
		}

		trace("ListArea.Draw: clearing row %d", y)
		printScreen(0, y, l.basicStyle.fg, l.basicStyle.bg, "", true)
	}

	var cached, written int
	var fgAttr, bgAttr termbox.Attribute
	for n := 0; n < perPage; n++ {
		switch {
		case n+currentPage.offset == l.currentLine:
			fgAttr = l.selectedStyle.fg
			bgAttr = l.selectedStyle.bg
		case l.SelectionContains(n + currentPage.offset):
			fgAttr = l.savedSelectionStyle.fg
			bgAttr = l.savedSelectionStyle.bg
		default:
			fgAttr = l.basicStyle.fg
			bgAttr = l.basicStyle.bg
		}

		if n >= bufsiz {
			break
		}

		if l.sortTopDown {
			y = n + start
		} else {
			y = start - n
		}

		target, err := buf.LineAt(n)
		if err != nil {
			break
		}

		if l.IsDirty() || target.IsDirty() {
			target.SetDirty(false)
		} else if l.displayCache[n] == target {
			cached++
			continue
		}

		written++
		l.displayCache[n] = target

		x := -l.currentCol
		xOffset := l.currentCol
		line := target.DisplayString()

		if l.IsSingleKeyJumpMode() {
			if n < len(l.config.SingleKeyJumpList) {
				printScreenWithOffset(x, y, xOffset, fgAttr|termbox.AttrBold|termbox.AttrReverse, bgAttr, fmt.Sprintf("%c", l.config.SingleKeyJumpList[n]), false)
				printScreenWithOffset(x + 1, y, xOffset, fgAttr, bgAttr, " ", false)
			} else {
				printScreenWithOffset(x, y, xOffset, fgAttr, bgAttr, "  ", false)
			}

			x += 2
		}

		matches := target.Indices()
		if matches == nil {
			printScreenWithOffset(x, y, xOffset, fgAttr, bgAttr, line, true)
			continue
		}

		prev := x
		index := 0

		for _, m := range matches {
			if m[0] > index {
				c := line[index:m[0]]
				n := printScreenWithOffset(prev, y, xOffset, fgAttr, bgAttr, c, false)
				prev += n
				index += len(c)
			}
			c := line[m[0]:m[1]]

			n := printScreenWithOffset(prev, y, xOffset, l.matchedStyle.fg, mergeAttribute(bgAttr, l.matchedStyle.bg), c, true)
			prev += n
			index += len(c)
		}

		m := matches[len(matches)-1]
		if m[0] > index {
			printScreenWithOffset(prev, y, xOffset, l.queryStyle.fg, mergeAttribute(bgAttr, l.queryStyle.bg), line[m[0]:m[1]], true)
		} else if len(line) > m[1] {
			printScreenWithOffset(prev, y, xOffset, fgAttr, bgAttr, line[m[1]:len(line)], true)
		}
	}
	l.SetDirty(false)
	trace("ListArea.Draw: Written total of %d lines (%d cached)\n", written+cached, cached)
}

// NewDefaultLayout creates a new Layout in the default format (top-down)
func NewDefaultLayout(ctx *Ctx) *BasicLayout {
	return &BasicLayout{
		Ctx:       ctx,
		StatusBar: NewStatusBar(ctx, AnchorBottom, 0+extraOffset),
		// The prompt is at the top
		prompt: NewUserPrompt(ctx, AnchorTop, 0),
		// The list area is at the top, after the prompt
		// It's also displayed top-to-bottom order
		list: NewListArea(ctx, AnchorTop, 1, true),
	}
}

// NewBottomUpLayout creates a new Layout in bottom-up format
func NewBottomUpLayout(ctx *Ctx) *BasicLayout {
	return &BasicLayout{
		Ctx:       ctx,
		StatusBar: NewStatusBar(ctx, AnchorBottom, 0+extraOffset),
		// The prompt is at the bottom, above the status bar
		prompt: NewUserPrompt(ctx, AnchorBottom, 1+extraOffset),
		// The list area is at the bottom, above the prompt
		// It's displayed in bottom-to-top order
		list: NewListArea(ctx, AnchorBottom, 2+extraOffset, false),
	}
}

func (l *BasicLayout) PurgeDisplayCache() {
	l.list.purgeDisplayCache()
}

// CalculatePage calculates which page we're displaying
func (l *BasicLayout) CalculatePage(perPage int) error {
	buf := l.GetCurrentLineBuffer()
	currentPage := l.currentPage
	currentPage.page = (l.currentLine / perPage) + 1
	currentPage.offset = (currentPage.page - 1) * perPage
	currentPage.perPage = perPage
	currentPage.total = buf.Size()

	trace("BasicLayout.CalculatePage: %#v", currentPage)
	if currentPage.total == 0 {
		currentPage.maxPage = 1
	} else {
		currentPage.maxPage = ((currentPage.total + perPage - 1) / perPage)
	}

	if currentPage.maxPage < currentPage.page {
		if buf.Size() == 0 {
			// wait for targets
			return fmt.Errorf("no targets or query. nothing to do")
		}
		l.currentLine = currentPage.offset
	}

	return nil
}

// DrawPrompt draws the prompt to the terminal
func (l *BasicLayout) DrawPrompt() {
	l.prompt.Draw()
}

// DrawScreen draws the entire screen
func (l *BasicLayout) DrawScreen(runningQuery bool) {
	trace("DrawScreen: START")
	defer trace("DrawScreen: END")

	perPage := linesPerPage()

	if err := l.CalculatePage(perPage); err != nil {
		return
	}

	l.DrawPrompt()
	l.list.Draw(l, perPage, runningQuery)

	if err := screen.Flush(); err != nil {
		return
	}
}

func linesPerPage() int {
	_, height := screen.Size()

	// list area is always the display area - 2 lines for prompt and status
	reservedLines := 2 + extraOffset
	return height - reservedLines
}

// MovePage scrolls the screen
func (l *BasicLayout) MovePage(p PagingRequest) (moved bool) {
	switch p {
	case ToScrollLeft, ToScrollRight:
		moved = horizontalScroll(l, p)
	default:
		moved = verticalScroll(l, p)
	}
	return
}

// verticalScroll moves the cursor position vertically
func verticalScroll(l *BasicLayout, p PagingRequest) bool {
	// Before we move, on which line were we located?
	lineBefore := l.currentLine

	defer func() { trace("currentLine changed from %d -> %d", lineBefore, l.currentLine) }()
	cp := l.currentPage
	buf := l.GetCurrentLineBuffer()
	lcur := buf.Size()

	defer func() {
		for _, lno := range []int{lineBefore, l.currentLine} {
			if oldLine, err := buf.LineAt(lno); err == nil {
				trace("Setting line %d dirty", lno)
				oldLine.SetDirty(true)
			}
		}
	}()

	lpp := linesPerPage()
	if l.list.sortTopDown {
		switch p.Type() {
		case ToLineAbove:
			l.currentLine--
		case ToLineBelow:
			l.currentLine++
		case ToScrollPageDown:
			l.currentLine += lpp
			if cp.page == cp.maxPage-1 && lcur < l.currentLine && (lcur-lineBefore) < lpp {
				l.currentLine = lcur - 1
			}
		case ToScrollPageUp:
			l.currentLine -= lpp
		case ToLineInPage:
			l.currentLine = cp.perPage * (cp.page - 1) + p.(JumpToLineRequest).Line()
		}
	} else {
		switch p {
		case ToLineAbove:
			l.currentLine++
		case ToLineBelow:
			l.currentLine--
		case ToScrollPageDown:
			l.currentLine -= lpp
		case ToScrollPageUp:
			l.currentLine += lpp
		case ToLineInPage:
			l.currentLine = cp.perPage * (cp.page - 1) - p.(JumpToLineRequest).Line()
		}
	}

	if l.currentLine < 0 {
		if lcur > 0 {
			// Go to last page, if possible
			l.currentLine = lcur - 1
		} else {
			l.currentLine = 0
		}
	} else if lcur > 0 && l.currentLine >= lcur {
		l.currentLine = 0
	}

	// if we were in range mode, we need to do stuff. otherwise
	// just bail out
	if !l.IsRangeMode() {
		return true
	}

	if l.list.sortTopDown {
		if l.currentLine < l.selectionRangeStart {
			for lineno := l.currentLine; lineno <= l.selectionRangeStart; lineno++ {
				l.SelectionAdd(lineno)
			}
			switch {
			case l.selectionRangeStart <= lineBefore:
				for lineno := l.selectionRangeStart; lineno <= lcur && lineno < lineBefore; lineno++ {
					l.SelectionRemove(lineno)
				}
			case lineBefore < l.currentLine:
				for lineno := lineBefore; lineno < l.currentLine; lineno++ {
					l.SelectionRemove(lineno)
				}
			}
		} else {
			for lineno := l.selectionRangeStart; lineno <= lcur && lineno <= l.currentLine; lineno++ {
				l.SelectionAdd(lineno)
			}

			switch {
			case lineBefore <= l.selectionRangeStart:
				for lineno := lineBefore; lineno < l.selectionRangeStart; lineno++ {
					l.SelectionRemove(lineno)
				}
			case l.currentLine < lineBefore:
				for lineno := l.currentLine; lineno <= lineBefore; lineno++ {
					l.SelectionRemove(lineno)
				}
			}
		}
	}

	return true
}

// horizontalScroll scrolls screen horizontal
func horizontalScroll(l *BasicLayout, p PagingRequest) bool {
	width, _ := screen.Size()

	if p == ToScrollRight {
		l.currentCol += width / 2
	} else if l.currentCol > 0 {
		l.currentCol -= width / 2
		if l.currentCol < 0 {
			l.currentCol = 0
		}
	} else {
		return false
	}

	l.list.SetDirty(true)

	return true
}
