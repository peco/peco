package peco

import (
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

// LayoutType describes the types of layout that peco can take
type LayoutType string

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

// VerticalAnchor describes the direction to which elements in the
// layout are anchored to
type VerticalAnchor int

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

// Layout represents the component that controls where elements are placed on screen
type Layout interface {
	PrintStatus(string, time.Duration)
	DrawPrompt()
	DrawScreen([]Line)
	MovePage(PagingRequest)
}

// Utility function
func mergeAttribute(a, b termbox.Attribute) termbox.Attribute {
	if a&0x0F == 0 || b&0x0F == 0 {
		return a | b
	}
	return ((a - 1) | (b - 1)) + 1
}

// Utility function
func printScreen(x, y int, fg, bg termbox.Attribute, msg string, fill bool) {
	for len(msg) > 0 {
		c, w := utf8.DecodeRuneInString(msg)
		if c == utf8.RuneError {
			c = '?'
			w = 1
		}
		msg = msg[w:]
		if c == '\t' {
			n := 4 - x % 4
			for i := 0; i <= n; i++ {
				screen.SetCell(x + i, y, ' ', fg, bg)
			}
			x += n
		} else {
			screen.SetCell(x, y, c, fg, bg)
			x += runewidth.RuneWidth(c)
		}
	}

	if !fill {
		return
	}

	width, _ := screen.Size()
	for ; x < width; x++ {
		screen.SetCell(x, y, ' ', fg, bg)
	}
}

// AnchorSettings groups items that are required to control
// where an anchored item is actually placed
type AnchorSettings struct {
	anchor       VerticalAnchor // AnchorTop or AnchorBottom
	anchorOffset int            // offset this many lines from the anchor
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

// UserPrompt draws the prompt line
type UserPrompt struct {
	*Ctx
	*AnchorSettings
	prefix    string
	prefixLen int
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
	}
}

// Draw draws the query prompt
func (u UserPrompt) Draw() {
	location := u.AnchorPosition()

	// print "QUERY>"
	printScreen(0, location, u.config.Style.BasicFG(), u.config.Style.BasicBG(), u.prefix, false)

	pos := u.CaretPos()
	if pos <= 0 { // XXX Do we really need this?
		u.SetCaretPos(0) // sanity
	}

	if pos > u.QueryLen() { // XXX Do we really need this?
		u.SetCaretPos(u.QueryLen())
	}

	if u.CaretPos() == u.QueryLen() {
		// the entire string + the caret after the string
		fg := u.config.Style.QueryFG()
		bg := u.config.Style.QueryBG()
		qs := u.QueryString()
		ql := runewidth.StringWidth(qs)
		printScreen(u.prefixLen+1, location, fg, bg, qs, false)
		printScreen(u.prefixLen+1+ql, location, fg|termbox.AttrReverse, bg|termbox.AttrReverse, " ", false)
		printScreen(u.prefixLen+1+ql+1, location, fg, bg, "", true)
	} else {
		// the caret is in the middle of the string
		prev := 0
		for i, r := range []rune(u.Query()) {
			fg := u.config.Style.QueryFG()
			bg := u.config.Style.QueryBG()
			if i == u.CaretPos() {
				fg |= termbox.AttrReverse
				bg |= termbox.AttrReverse
			}
			screen.SetCell(u.prefixLen+1+prev, location, r, fg, bg)
			prev += runewidth.RuneWidth(r)
		}
	}

	width, _ := screen.Size()

	pmsg := fmt.Sprintf("%s [%d (%d/%d)]", u.Matcher().String(), u.currentPage.total, u.currentPage.index, u.currentPage.maxPage)
	printScreen(width-runewidth.StringWidth(pmsg), location, u.config.Style.BasicFG(), u.config.Style.BasicBG(), pmsg, false)
}

// StatusBar draws the status message bar
type StatusBar struct {
	*Ctx
	*AnchorSettings
	clearTimer *time.Timer
	timerMutex sync.Locker
}

// NewStatusBar creates a new StatusBar struct
func NewStatusBar(ctx *Ctx, anchor VerticalAnchor, anchorOffset int) *StatusBar {
	return &StatusBar{
		ctx,
		NewAnchorSettings(anchor, anchorOffset),
		nil,
		newMutex(),
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

	fgAttr := s.config.Style.BasicFG()
	bgAttr := s.config.Style.BasicBG()

	if w > width {
		printScreen(0, location, fgAttr, bgAttr, string(pad), false)
	}

	if width > 0 {
		printScreen(w-width, location, fgAttr|termbox.AttrReverse|termbox.AttrBold, bgAttr|termbox.AttrReverse, msg, false)
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

// ListArea represents the area where the actual line buffer is
// displayed in the screen
type ListArea struct {
	*Ctx
	*AnchorSettings
	sortTopDown bool
}

// NewListArea creates a new ListArea struct
func NewListArea(ctx *Ctx, anchor VerticalAnchor, anchorOffset int, sortTopDown bool) *ListArea {
	return &ListArea{
		ctx,
		NewAnchorSettings(anchor, anchorOffset),
		sortTopDown,
	}
}

// Draw displays the ListArea on the screen
func (l *ListArea) Draw(targets []Line, perPage int) {
	currentPage := l.currentPage

	start := l.AnchorPosition()

	var y int
	var fgAttr, bgAttr termbox.Attribute
	for n := 0; n < perPage; n++ {
		switch {
		case n+currentPage.offset == l.currentLine-1:
			fgAttr = l.config.Style.SelectedFG()
			bgAttr = l.config.Style.SelectedBG()
		case l.SelectionContains(n + currentPage.offset + 1):
			fgAttr = l.config.Style.SavedSelectionFG()
			bgAttr = l.config.Style.SavedSelectionBG()
		default:
			fgAttr = l.config.Style.BasicFG()
			bgAttr = l.config.Style.BasicBG()
		}

		targetIdx := currentPage.offset + n
		if targetIdx >= len(targets) {
			break
		}

		if l.sortTopDown {
			y = n + start
		} else {
			y = start - n
		}

		target := targets[targetIdx]
		line := target.DisplayString()
		matches := target.Indices()
		if matches == nil {
			printScreen(0, y, fgAttr, bgAttr, line, true)
		} else {
			prev := 0
			index := 0
			for _, m := range matches {
				if m[0] > index {
					c := line[index:m[0]]
					printScreen(prev, y, fgAttr, bgAttr, c, false)
					prev += runewidth.StringWidth(c)
					index += len(c)
				}
				c := line[m[0]:m[1]]
				printScreen(prev, y, l.config.Style.MatchedFG(), mergeAttribute(bgAttr, l.config.Style.MatchedBG()), c, true)
				prev += runewidth.StringWidth(c)
				index += len(c)
			}

			m := matches[len(matches)-1]
			if m[0] > index {
				printScreen(prev, y, l.config.Style.QueryFG(), mergeAttribute(bgAttr, l.config.Style.QueryBG()), line[m[0]:m[1]], true)
			} else if len(line) > m[1] {
				printScreen(prev, y, fgAttr, bgAttr, line[m[1]:len(line)], true)
			}
		}
	}
}

// BasicLayout is... the basic layout :) At this point this is the
// only struct for layouts, which means that while the position
// of components may be configurable, the actual types of components
// that are used are set and static
type BasicLayout struct {
	*Ctx
	*StatusBar
	prompt *UserPrompt
	list   *ListArea
}

// NewDefaultLayout creates a new Layout in the default format (top-down)
func NewDefaultLayout(ctx *Ctx) *BasicLayout {
	return &BasicLayout{
		Ctx:       ctx,
		StatusBar: NewStatusBar(ctx, AnchorBottom, 0),
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
		StatusBar: NewStatusBar(ctx, AnchorBottom, 0),
		// The prompt is at the bottom, above the status bar
		prompt: NewUserPrompt(ctx, AnchorBottom, 1),
		// The list area is at the bottom, above the prompt
		// IT's displayed in bottom-to-top order
		list: NewListArea(ctx, AnchorBottom, 2, false),
	}
}

// CalculatePage calculates which page we're displaying
func (l *BasicLayout) CalculatePage(targets []Line, perPage int) error {
CALCULATE_PAGE:
	currentPage := l.currentPage
	currentPage.index = ((l.currentLine - 1) / perPage) + 1
	if currentPage.index <= 0 {
		currentPage.index = 1
	}
	currentPage.offset = (currentPage.index - 1) * perPage
	currentPage.perPage = perPage
	currentPage.total = len(targets)
	if currentPage.total == 0 {
		currentPage.maxPage = 1
	} else {
		currentPage.maxPage = ((currentPage.total + perPage - 1) / perPage)
	}

	if currentPage.maxPage < currentPage.index {
		if len(targets) == 0 && l.QueryLen() == 0 {
			// wait for targets
			return fmt.Errorf("no targets or query. nothing to do")
		}
		l.currentLine = currentPage.offset
		goto CALCULATE_PAGE
	}

	return nil
}

func (l *BasicLayout) DrawPrompt() {
	l.prompt.Draw()
}

// DrawScreen draws the entire screen
func (l *BasicLayout) DrawScreen(targets []Line) {
	if err := screen.Clear(l.config.Style.BasicFG(), l.config.Style.BasicBG()); err != nil {
		return
	}

	if l.currentLine > len(targets) && len(targets) > 0 {
		l.currentLine = len(targets)
	}

	perPage := linesPerPage()

	if err := l.CalculatePage(targets, perPage); err != nil {
		return
	}

	l.DrawPrompt()
	l.list.Draw(targets, perPage)

	if err := screen.Flush(); err != nil {
		return
	}
}

func linesPerPage() int {
	_, height := screen.Size()
	return height - 2 // list area is always the display area - 2 lines for prompt and status
}

// MovePage moves the cursor
func (l *BasicLayout) MovePage(p PagingRequest) {
	// Before we moved, on which line were we located?
	lineBefore := l.currentLine
	if l.list.sortTopDown {
		switch p {
		case ToLineAbove:
			l.currentLine--
		case ToLineBelow:
			l.currentLine++
		case ToScrollPageDown:
			l.currentLine += linesPerPage()
		case ToScrollPageUp:
			l.currentLine -= linesPerPage()
		}
	} else {
		switch p {
		case ToLineAbove:
			l.currentLine++
		case ToLineBelow:
			l.currentLine--
		case ToScrollPageDown:
			l.currentLine -= linesPerPage()
		case ToScrollPageUp:
			l.currentLine += linesPerPage()
		}
	}

	lcur := len(l.current)
	if l.currentLine < 1 {
		if l.current != nil {
			// Go to last page, if possible
			l.currentLine = lcur
		} else {
			l.currentLine = 1
		}
	} else if l.current != nil && l.currentLine > lcur {
		l.currentLine = 1
	}

	// if we were in range mode, we need to do stuff. otherwise
	// just bail out
	if !l.IsRangeMode() {
		return
	}

	if l.list.sortTopDown {
		if l.currentLine < l.selectionRangeStart {
			for lineno := l.currentLine; lineno <= l.selectionRangeStart; lineno++ {
				l.SelectionAdd(lineno)
			}
			switch {
			case l.selectionRangeStart <= lineBefore:
				for lineno := l.selectionRangeStart + 1; lineno <= lcur && lineno < lineBefore; lineno++ {
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
				for lineno := l.currentLine + 1; lineno <= lineBefore; lineno++ {
					l.SelectionRemove(lineno)
				}
			}
		}
	}
}
