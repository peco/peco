package peco

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
	"github.com/pkg/errors"
)

var extraOffset = 0

const (
	DefaultLayoutType = LayoutTypeTopDown
	// LayoutTypeTopDown makes the layout so the items read from top to bottom
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
func NewUserPrompt(anchor VerticalAnchor, anchorOffset int, prompt string, styles *StyleSet) *UserPrompt {
	if len(prompt) <= 0 { // default
		prompt = "QUERY>"
	}
	promptLen := runewidth.StringWidth(prompt)

	return &UserPrompt{
		AnchorSettings: &AnchorSettings{anchor, anchorOffset},
		prompt:         prompt,
		promptLen:      promptLen,
		styles:					styles,
	}
}

// Draw draws the query prompt
func (u UserPrompt) Draw(state *Peco) {
	trace("UserPrompt.Draw: START")
	defer trace("UserPrompt.Draw: END")

	location := u.AnchorPosition()

	// print "QUERY>"
	printScreen(0, location, u.styles.Basic.fg, u.styles.Basic.bg, u.prompt, false)

	c := state.Caret()
	if c.Pos() <= 0 { // XXX Do we really need this?
		c.SetPos(0) // sanity
	}

	q := state.Query()
	qs := q.String()
	ql := q.Len()
	if c.Pos() > ql { // XXX Do we really need this?
		c.SetPos(ql)
	}

	fg := u.styles.Query.fg
	bg := u.styles.Query.bg
	switch ql {
	case 0:
		printScreen(u.promptLen, location, fg, bg, "", true)
		printScreen(u.promptLen+1, location, fg|termbox.AttrReverse, bg|termbox.AttrReverse, " ", false)
	case c.Pos():
		// the entire string + the caret after the string
		printScreen(u.promptLen, location, fg, bg, "", true)
		printScreen(u.promptLen+1, location, fg, bg, qs, false)
		printScreen(u.promptLen+runewidth.StringWidth(qs)+1, location, fg|termbox.AttrReverse, bg|termbox.AttrReverse, " ", false)
	default:
		// the caret is in the middle of the string
		prev := 0
		for i, r := range q.Runes() {
			fg := u.styles.Query.fg
			bg := u.styles.Query.bg
			if i == c.Pos() {
				fg |= termbox.AttrReverse
				bg |= termbox.AttrReverse
			}
			screen.SetCell(u.promptLen+1+prev, location, r, fg, bg)
			prev += runewidth.RuneWidth(r)
		}
		fg := u.styles.Query.fg
		bg := u.styles.Query.bg
		printScreen(u.promptLen+prev+1, location, fg, bg, "", true)
	}

	width, _ := screen.Size()

	loc := state.Location()
	pmsg := fmt.Sprintf("%s [%d (%d/%d)]", state.Filters().GetCurrent().String(), loc.Total(), loc.Page(), loc.MaxPage())
	printScreen(width-runewidth.StringWidth(pmsg), location, u.styles.Basic.fg, u.styles.Basic.bg, pmsg, false)

	screen.Flush()
}

// NewStatusBar creates a new StatusBar struct
func NewStatusBar(anchor VerticalAnchor, anchorOffset int, styles *StyleSet) *StatusBar {
	return &StatusBar{
		AnchorSettings: NewAnchorSettings(anchor, anchorOffset),
		clearTimer:     nil,
		styles:     styles,
		timerMutex:     newMutex(),
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

	fgAttr := s.styles.Basic.fg
	bgAttr := s.styles.Basic.bg

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
func NewListArea(anchor VerticalAnchor, anchorOffset int, sortTopDown bool, styles *StyleSet) *ListArea {
	return &ListArea{
		AnchorSettings:      NewAnchorSettings(anchor, anchorOffset),
		displayCache:        []Line{},
		dirty:               false,
		sortTopDown:         sortTopDown,
		styles:              styles,
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

func selectionContains(state *Peco, n int) bool {
	if l, err := state.CurrentLineBuffer().LineAt(n); err == nil {
		return state.Selection().Has(l)
	}
	return false
}

// Draw displays the ListArea on the screen
func (l *ListArea) Draw(state *Peco, parent Layout, perPage int, runningQuery bool) {
	trace("ListArea.Draw: START")
	defer trace("ListArea.Draw: END")
	loc := state.Location()

	linebuf := state.CurrentLineBuffer()

	// Should only get into this clause if we are RUNNING A QUERY.
	// regular paging shouldn't be affected. This clause basically
	// makes sure that we never have an empty screen when we are
	// at a large enough page, but we don't have enough entries
	// to fill that many pages in the buffer
	if runningQuery {
		bufsiz := linebuf.Size()
		page := loc.Page()

		for page > 1 {
			if (loc.PerPage()*(page-1) < bufsiz) &&
				(loc.PerPage()*page) >= bufsiz {
				break
			}

			page--
		}
		if loc.Page() != page {
			loc.SetPage(page)
			parent.DrawPrompt(state)
		}
	}

	pf := loc.PageCrop()
	buf := pf.Crop(linebuf)
	bufsiz := buf.Size()

	// This protects us from losing the selected line in case our selected
	// line is greater than the buffer
	if lbufsiz := linebuf.Size(); lbufsiz > 0 && loc.LineNumber() >= lbufsiz {
		loc.SetLineNumber(lbufsiz - 1)
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
		printScreen(0, y, l.styles.Basic.fg, l.styles.Basic.bg, "", true)
	}

	var cached, written int
	var fgAttr, bgAttr termbox.Attribute
	for n := 0; n < perPage; n++ {
		switch {
		case n+loc.Offset() == loc.LineNumber():
			fgAttr = l.styles.Selected.fg
			bgAttr = l.styles.Selected.bg
		case selectionContains(state, n + loc.Offset()):
			fgAttr = l.styles.SavedSelection.fg
			bgAttr = l.styles.SavedSelection.bg
		default:
			fgAttr = l.styles.Basic.fg
			bgAttr = l.styles.Basic.bg
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

		x := -1 * loc.Column()
		xOffset := loc.Column()
		line := target.DisplayString()

		if state.SingleKeyJumpMode() || state.SingleKeyJumpShowPrefix() {
			prefixes := state.SingleKeyJumpPrefixes()
			if n < len(prefixes) {
				printScreenWithOffset(x, y, xOffset, fgAttr|termbox.AttrBold|termbox.AttrReverse, bgAttr, string(prefixes[n]), false)
				printScreenWithOffset(x+1, y, xOffset, fgAttr, bgAttr, " ", false)
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

			n := printScreenWithOffset(prev, y, xOffset, l.styles.Matched.fg, mergeAttribute(bgAttr, l.styles.Matched.bg), c, true)
			prev += n
			index += len(c)
		}

		m := matches[len(matches)-1]
		if m[0] > index {
			printScreenWithOffset(prev, y, xOffset, l.styles.Query.fg, mergeAttribute(bgAttr, l.styles.Query.bg), line[m[0]:m[1]], true)
		} else if len(line) > m[1] {
			printScreenWithOffset(prev, y, xOffset, fgAttr, bgAttr, line[m[1]:len(line)], true)
		}
	}
	l.SetDirty(false)
	trace("ListArea.Draw: Written total of %d lines (%d cached)\n", written+cached, cached)
}

// NewDefaultLayout creates a new Layout in the default format (top-down)
func NewDefaultLayout(state *Peco) *BasicLayout {
	return &BasicLayout{
		StatusBar: NewStatusBar(AnchorBottom, 0+extraOffset, state.Styles()),
		// The prompt is at the top
		prompt: NewUserPrompt(AnchorTop, 0, state.Prompt(), state.Styles()),
		// The list area is at the top, after the prompt
		// It's also displayed top-to-bottom order
		list: NewListArea(AnchorTop, 1, true, state.Styles()),
	}
}

// NewBottomUpLayout creates a new Layout in bottom-up format
func NewBottomUpLayout(state *Peco) *BasicLayout {
	return &BasicLayout{
		StatusBar: NewStatusBar(AnchorBottom, 0+extraOffset, state.Styles()),
		// The prompt is at the bottom, above the status bar
		prompt: NewUserPrompt(AnchorBottom, 1+extraOffset, state.Prompt(), state.Styles()),
		// The list area is at the bottom, above the prompt
		// It's displayed in bottom-to-top order
		list: NewListArea(AnchorBottom, 2+extraOffset, false, state.Styles()),
	}
}

func (l *BasicLayout) PurgeDisplayCache() {
	l.list.purgeDisplayCache()
}

// CalculatePage calculates which page we're displaying
func (l *BasicLayout) CalculatePage(state *Peco, perPage int) error {
	buf := state.CurrentLineBuffer()
	loc := state.Location()
	loc.SetPage((loc.LineNumber() / perPage) + 1)
	loc.SetOffset((loc.Page() - 1) * perPage)
	loc.SetPerPage(perPage)
	loc.SetTotal(buf.Size())

	trace("BasicLayout.CalculatePage: %#v", loc)
	if loc.Total() == 0 {
		loc.SetMaxPage(1)
	} else {
		loc.SetMaxPage((loc.Total() + perPage - 1) / perPage)
	}

	if loc.MaxPage() < loc.Page() {
		if buf.Size() == 0 {
			// wait for targets
			return errors.New("no targets or query. nothing to do")
		}
		loc.SetLineNumber(loc.Offset())
	}

	return nil
}

// DrawPrompt draws the prompt to the terminal
func (l *BasicLayout) DrawPrompt(state *Peco) {
	l.prompt.Draw(state)
}

// DrawScreen draws the entire screen
func (l *BasicLayout) DrawScreen(state *Peco, runningQuery bool) {
	trace("DrawScreen: START")
	defer trace("DrawScreen: END")

	perPage := linesPerPage()

	if err := l.CalculatePage(state, perPage); err != nil {
		return
	}

	l.DrawPrompt(state)
	l.list.Draw(state, l, perPage, runningQuery)

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
func (l *BasicLayout) MovePage(state *Peco, p PagingRequest) (moved bool) {
	switch p.Type() {
	case ToScrollLeft, ToScrollRight:
		moved = horizontalScroll(state, l, p)
	default:
		moved = verticalScroll(state, l, p)
	}
	return
}

// verticalScroll moves the cursor position vertically
func verticalScroll(state *Peco, l *BasicLayout, p PagingRequest) bool {
	// Before we move, on which line were we located?
	loc := state.Location()
	lineBefore := loc.LineNumber()
	lineno := lineBefore

	defer func() { trace("currentLine changed from %d -> %d", lineBefore, state.Location().LineNumber()) }()
	buf := state.CurrentLineBuffer()
	lcur := buf.Size()

	defer func() {
		for _, lno := range []int{lineBefore, loc.LineNumber()} {
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
			lineno--
		case ToLineBelow:
			lineno++
		case ToScrollPageDown:
			lineno += lpp
			if loc.Page() == loc.MaxPage()-1 && lcur < lineno && (lcur-lineBefore) < lpp {
				lineno = lcur - 1
			}
		case ToScrollPageUp:
			lineno -= lpp
		case ToLineInPage:
			lineno = loc.PerPage()*(loc.Page()-1) + p.(JumpToLineRequest).Line()
		}
	} else {
		switch p.Type() {
		case ToLineAbove:
			lineno++
		case ToLineBelow:
			lineno--
		case ToScrollPageDown:
			lineno -= lpp
		case ToScrollPageUp:
			lineno += lpp
		case ToLineInPage:
			lineno = loc.PerPage()*(loc.Page()-1) - p.(JumpToLineRequest).Line()
		}
	}

	if lineno < 0 {
		if lcur > 0 {
			// Go to last page, if possible
			lineno = lcur - 1
		} else {
			lineno = 0
		}
	} else if lcur > 0 && lineno >= lcur {
		lineno = 0
	}

	// XXX DO NOT RETURN UNTIL YOU SET THE LINE NUMBER HERE
	loc.SetLineNumber(lineno)

	// if we were in range mode, we need to do stuff. otherwise
	// just bail out
	if !state.RangeMode() {
		return true
	}

	sel := state.Selection()
	if l.list.sortTopDown {
		if loc.LineNumber() < state.SelectionRangeStart() {
			for lineno := loc.LineNumber(); lineno <= state.SelectionRangeStart(); lineno++ {
				if line, err := buf.LineAt(lineno); err == nil {
					sel.Add(line)
				}
			}
			switch {
			case state.SelectionRangeStart() <= lineBefore:
				for lineno := state.SelectionRangeStart(); lineno <= lcur && lineno < lineBefore; lineno++ {
					if line, err := buf.LineAt(lineno); err == nil {
						sel.Remove(line)
					}
				}
			case lineBefore < loc.LineNumber():
				for lineno := lineBefore; lineno < loc.LineNumber(); lineno++ {
					if line, err := buf.LineAt(lineno); err == nil {
						sel.Remove(line)
					}
				}
			}
		} else {
			for lineno := state.SelectionRangeStart(); lineno <= lcur && lineno <= loc.LineNumber(); lineno++ {
				if line, err := buf.LineAt(lineno); err == nil {
					sel.Add(line)
				}
			}

			switch {
			case lineBefore <= state.SelectionRangeStart():
				for lineno := lineBefore; lineno < state.SelectionRangeStart(); lineno++ {
					if line, err := buf.LineAt(lineno); err == nil {
						sel.Remove(line)
					}
				}
			case loc.LineNumber() < lineBefore:
				for lineno := loc.LineNumber(); lineno <= lineBefore; lineno++ {
					if line, err := buf.LineAt(lineno); err == nil {
						sel.Remove(line)
					}
				}
			}
		}
	}

	return true
}

// horizontalScroll scrolls screen horizontal
func horizontalScroll(state *Peco, l *BasicLayout, p PagingRequest) bool {
	width, _ := screen.Size()
	loc := state.Location()
	if p.Type() == ToScrollRight {
		loc.SetColumn(loc.Column() + width / 2)
	} else if loc.Column() > 0 {
		loc.SetColumn(loc.Column() - width / 2)
		if loc.Column() < 0 {
			loc.SetColumn(0)
		}
	} else {
		return false
	}

	l.list.SetDirty(true)

	return true
}
