package peco

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

type LayoutType string
const (
	LayoutTypeTopDown = "top-down"
	LayoutTypeBottomUp = "bottom-up"
)

// IsValidLayoutType checks if a string is a supported layout type
func IsValidLayoutType(v string) bool {
	if v == LayoutTypeTopDown || v == LayoutTypeBottomUp {
		return true
	}

	return false
}

type VerticalAnchor int

const (
	AnchorTop VerticalAnchor = iota + 1
	AnchorBottom
)

type Layout interface {
	ClearStatus(time.Duration)
	PrintStatus(string)
	DrawScreen([]Match)
	MovePage(PagingRequest)
}

// Utility function
func mergeAttribute(a, b termbox.Attribute) termbox.Attribute {
	if a&0x0F == 0 || b&0x0F == 0 {
		return a | b
	} else {
		return ((a - 1) | (b - 1)) + 1
	}
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
		termbox.SetCell(x, y, c, fg, bg)
		x += runewidth.RuneWidth(c)
	}

	if !fill {
		return
	}

	width, _ := termbox.Size()
	for ; x < width; x++ {
		termbox.SetCell(x, y, ' ', fg, bg)
	}
}

type AnchorSettings struct {
	anchor       VerticalAnchor // AnchorTop or AnchorBottom
	anchorOffset int            // offset this many lines from the anchor
}

// AnchorPosition returns the starting y-offset, based on the
// anchor type and offset
func (as AnchorSettings) AnchorPosition() int {
	var pos int
	switch as.anchor {
	case AnchorTop:
		pos = as.anchorOffset
	case AnchorBottom:
		_, h := termbox.Size()
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

func (u UserPrompt) Draw() {
	location := u.AnchorPosition()

	// print "QUERY>"
	printScreen(0, location, u.config.Style.BasicFG(), u.config.Style.BasicBG(), u.prefix, false)

	if u.caretPos <= 0 {
		u.caretPos = 0 // sanity
	}

	if u.caretPos > len(u.query) {
		u.caretPos = len(u.query)
	}

	if u.caretPos == len(u.query) {
		// the entire string + the caret after the string
		fg := u.config.Style.QueryFG()
		bg := u.config.Style.QueryBG()
		qs := string(u.query)
		ql := runewidth.StringWidth(qs)
		printScreen(u.prefixLen+1, location, fg, bg, qs, false)
		printScreen(u.prefixLen+1+ql, location, fg|termbox.AttrReverse, bg|termbox.AttrReverse, " ", false)
		printScreen(u.prefixLen+1+ql+1, location, fg, bg, "", true)
	} else {
		// the caret is in the middle of the string
		prev := 0
		fg := u.config.Style.QueryFG()
		bg := u.config.Style.QueryBG()
		for i, r := range u.query {
			if i == u.caretPos {
				fg |= termbox.AttrReverse
				bg |= termbox.AttrReverse
			}
			termbox.SetCell(u.prefixLen+1+prev, location, r, fg, bg)
			prev += runewidth.RuneWidth(r)
		}
	}

	width, _ := termbox.Size()

	pmsg := fmt.Sprintf("%s [%d/%d]", u.Matcher().String(), u.currentPage.index, u.maxPage)
	printScreen(width-runewidth.StringWidth(pmsg), location, u.config.Style.BasicFG(), u.config.Style.BasicBG(), pmsg, false)
}

// StatusBar draws the status message bar
type StatusBar struct {
	*Ctx
	*AnchorSettings
	clearTimer *time.Timer
}

func NewStatusBar(ctx *Ctx, anchor VerticalAnchor, anchorOffset int) *StatusBar {
	return &StatusBar{
		ctx,
		&AnchorSettings{ anchor, anchorOffset },
		nil,
	}
}

func (s *StatusBar) stopTimer() {
	if t := s.clearTimer; t != nil {
		t.Stop()
	}
}

func (s *StatusBar) ClearStatus(d time.Duration) {
	s.stopTimer()
	s.clearTimer = time.AfterFunc(d, func() {
		s.PrintStatus("")
	})
}

func (s *StatusBar) PrintStatus(msg string) {
	s.stopTimer()

	location := s.AnchorPosition()

	w, _ := termbox.Size()
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
	termbox.Flush()
}

type ListArea struct {
	*Ctx
	*AnchorSettings
	sortTopDown  bool
}

func NewListArea(ctx *Ctx, anchor VerticalAnchor, anchorOffset int, sortTopDown bool) *ListArea {
	return &ListArea{
		ctx,
		&AnchorSettings{ anchor, anchorOffset },
		sortTopDown,
	}
}

func (l *ListArea) Draw(targets []Match, perPage int) {
	currentPage := l.currentPage

	start := l.AnchorPosition()

	var y int
	var fgAttr, bgAttr termbox.Attribute
	for n := 0; n < perPage; n++ {
		switch {
		case n+currentPage.offset == l.currentLine-1:
			fgAttr = l.config.Style.SelectedFG()
			bgAttr = l.config.Style.SelectedBG()
		case l.selection.Has(n+currentPage.offset) || l.SelectedRange().Has(n+currentPage.offset):
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
		line := target.Line()
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

func (l *BasicLayout) CalculatePage(targets []Match, perPage int) error {
CALCULATE_PAGE:
	currentPage := l.currentPage
	currentPage.index = ((l.currentLine - 1) / perPage) + 1
	if currentPage.index <= 0 {
		currentPage.index = 1
	}
	currentPage.offset = (currentPage.index - 1) * perPage
	currentPage.perPage = perPage
	if len(targets) == 0 {
		l.maxPage = 1
	} else {
		l.maxPage = ((len(targets) + perPage - 1) / perPage)
	}

	if l.maxPage < currentPage.index {
		if len(targets) == 0 && len(l.query) == 0 {
			// wait for targets
			return fmt.Errorf("no targets or query. nothing to do")
		}
		l.currentLine = currentPage.offset
		goto CALCULATE_PAGE
	}

	return nil
}

func (l *BasicLayout) DrawScreen(targets []Match) {
	if err := termbox.Clear(l.config.Style.BasicFG(), l.config.Style.BasicBG()); err != nil {
		return
	}

	if l.currentLine > len(targets) && len(targets) > 0 {
		l.currentLine = len(targets)
	}

	_, height := termbox.Size()
	perPage := height - 2 // list area is always the display area - 2 lines for prompt and status

	if err := l.CalculatePage(targets, perPage); err != nil {
		return
	}

	l.prompt.Draw()
	l.list.Draw(targets, perPage)

	if err := termbox.Flush(); err != nil {
		return
	}
}

func (l *BasicLayout) MovePage(p PagingRequest) {
	_, height := termbox.Size()
	perPage := height - 2 // list area is always the display area - 2 lines for prompt and status

	switch p {
	case ToLineAbove:
		if l.list.sortTopDown {
			l.currentLine--
		} else {
			l.currentLine++
		}
	case ToLineBelow:
		if l.list.sortTopDown {
			l.currentLine++
		} else {
			l.currentLine--
		}
	case ToPrevPage, ToNextPage:
		if p == ToPrevPage {
			l.currentLine -= perPage
		} else {
			l.currentLine += perPage
		}
	}

	if l.currentLine < 1 {
		if l.current != nil {
			// Go to last page, if possible
			l.currentLine = len(l.current)
		} else {
			l.currentLine = 1
		}
	} else if l.current != nil && l.currentLine > len(l.current) {
		l.currentLine = 1
	}
}

