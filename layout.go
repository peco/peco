package peco

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

type Layout interface {
	ClearStatus(time.Duration)
	PrintStatus(string)
	DrawScreen([]Match)
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

// UserPrompt draws the prompt line
type UserPrompt struct {
	*Ctx
	location int
	prefix string
	prefixLen int
}

func NewUserPrompt(ctx *Ctx) *UserPrompt {
	prefix := ctx.config.Prompt
	if len(prefix) <= 0 { // default
		prefix = "QUERY>"
	}
	prefixLen := runewidth.StringWidth(prefix)

	return &UserPrompt{
		Ctx: ctx,
		location:  0, // effectively, the line number where the prompt is going to be displayed at
		prefix:    prefix,
		prefixLen: prefixLen,
	}
}

func (u UserPrompt) Draw() {
	// print "QUERY>"
	printScreen(0, u.location, u.config.Style.BasicFG(), u.config.Style.BasicBG(), u.prefix, false)

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
		printScreen(u.prefixLen+1, u.location, fg, bg, qs, false)
		printScreen(u.prefixLen+1+ql, u.location, fg|termbox.AttrReverse, bg|termbox.AttrReverse, " ", false)
		printScreen(u.prefixLen+1+ql+1, u.location, fg, bg, "", true)
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
			termbox.SetCell(u.prefixLen+1+prev, u.location, r, fg, bg)
			prev += runewidth.RuneWidth(r)
		}
	}

	width, _ := termbox.Size()

	pmsg := fmt.Sprintf("%s [%d/%d]", u.Matcher().String(), u.currentPage.index, u.maxPage)
	printScreen(width-runewidth.StringWidth(pmsg), u.location, u.config.Style.BasicFG(), u.config.Style.BasicBG(), pmsg, false)
}

// StatusBar draws the status message bar
type StatusBar struct {
	*Ctx
	clearTimer *time.Timer
}

func NewStatusBar(ctx *Ctx) *StatusBar {
	return &StatusBar{
		ctx,
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

	w, h := termbox.Size()

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
		printScreen(0, h-2, fgAttr, bgAttr, string(pad), false)
	}

	if width > 0 {
		printScreen(w-width, h-2, fgAttr|termbox.AttrReverse|termbox.AttrBold, bgAttr|termbox.AttrReverse, msg, false)
	}
	termbox.Flush()
}

type ListArea struct {
	*Ctx
	sortTopDown bool
}

func NewListArea(ctx *Ctx) *ListArea {
	return &ListArea{
		ctx,
		true,
	}
}

func (l *ListArea) Draw(targets []Match, perPage int) {
	currentPage := l.currentPage

	var fgAttr, bgAttr termbox.Attribute
	for n := 1; n <= perPage; n++ {
		switch {
		case n+currentPage.offset == l.currentLine:
			fgAttr = l.config.Style.SelectedFG()
			bgAttr = l.config.Style.SelectedBG()
		case l.selection.Has(n+currentPage.offset) || l.SelectedRange().Has(n+currentPage.offset):
			fgAttr = l.config.Style.SavedSelectionFG()
			bgAttr = l.config.Style.SavedSelectionBG()
		default:
			fgAttr = l.config.Style.BasicFG()
			bgAttr = l.config.Style.BasicBG()
		}

		targetIdx := currentPage.offset + n - 1
		if targetIdx >= len(targets) {
			break
		}

		target := targets[targetIdx]
		line := target.Line()
		matches := target.Indices()
		if matches == nil {
			printScreen(0, n, fgAttr, bgAttr, line, true)
		} else {
			prev := 0
			index := 0
			for _, m := range matches {
				if m[0] > index {
					c := line[index:m[0]]
					printScreen(prev, n, fgAttr, bgAttr, c, false)
					prev += runewidth.StringWidth(c)
					index += len(c)
				}
				c := line[m[0]:m[1]]
				printScreen(prev, n, l.config.Style.MatchedFG(), mergeAttribute(bgAttr, l.config.Style.MatchedBG()), c, true)
				prev += runewidth.StringWidth(c)
				index += len(c)
			}

			m := matches[len(matches)-1]
			if m[0] > index {
				printScreen(prev, n, l.config.Style.QueryFG(), mergeAttribute(bgAttr, l.config.Style.QueryBG()), line[m[0]:m[1]], true)
			} else if len(line) > m[1] {
				printScreen(prev, n, fgAttr, bgAttr, line[m[1]:len(line)], true)
			}
		}
	}
}

type basicLayout struct {
	*Ctx
	*StatusBar
	prompt *UserPrompt
	list   *ListArea
}

// DefaultLayout implements the top-down layout
type DefaultLayout struct {
	*basicLayout
}
type BottomUpLayout struct {
	*basicLayout
}

func NewDefaultLayout(ctx *Ctx) *DefaultLayout {
	return &DefaultLayout{
		&basicLayout{
			Ctx: ctx,
			StatusBar: NewStatusBar(ctx),
			prompt: NewUserPrompt(ctx),
			list: NewListArea(ctx),
		},
	}
}

func (l *DefaultLayout) CalculatePage(targets []Match, perPage int) error {
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

func (l *DefaultLayout) DrawScreen(targets []Match) {
	fgAttr := l.config.Style.BasicFG()
	bgAttr := l.config.Style.BasicBG()

	if err := termbox.Clear(fgAttr, bgAttr); err != nil {
		return
	}

	if l.currentLine > len(targets) && len(targets) > 0 {
		l.currentLine = len(targets)
	}

	_, height := termbox.Size()
	perPage := height - 4

	if err := l.CalculatePage(targets, perPage); err != nil {
		return
	}

	l.prompt.Draw()
	l.list.Draw(targets, perPage)
	if err := termbox.Flush(); err != nil {
		return
	}
}
