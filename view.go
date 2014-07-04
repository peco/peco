package peco

import (
	"fmt"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

// View handles the drawing/updating the screen
type View struct {
	*Ctx
}

// PagingRequest can be sent to move the selection cursor
type PagingRequest int

const (
	// ToNextLine moves the selection to the next line
	ToNextLine PagingRequest = iota
	// ToNextPage moves the selection to the next page
	ToNextPage
	// ToPrevLine moves the selection to the previous line
	ToPrevLine
	// ToPrevPage moves the selection to the previous page
	ToPrevPage
)

// Loop receives requests to update the screen
func (v *View) Loop() {
	defer v.ReleaseWaitGroup()
	for {
		select {
		case <-v.LoopCh():
			return
		case m := <-v.StatusMsgCh():
			v.printStatus(m)
		case r := <-v.PagingCh():
			v.movePage(r)
		case lines := <-v.DrawCh():
			v.drawScreen(lines)
		}
	}
}

func (v *View) printStatus(msg string) {
	w, h := termbox.Size()

	width := runewidth.StringWidth(msg)
	if width > w {
		msg = runewidth.Truncate(msg, width - w, "")
	}

	pad := make([]byte, w-width)
	for i := 0; i < w-width; i++ {
		pad[i] = ' '
	}

	fgAttr := v.config.Style.Basic.fg
	bgAttr := v.config.Style.Basic.bg

	printTB(0, h-2, fgAttr, bgAttr, string(pad))
	if width > 0 {
		printTB(w-width, h-2, fgAttr|termbox.AttrReverse|termbox.AttrBold, bgAttr|termbox.AttrReverse, msg)
	}
	termbox.Flush()
}

func printTB(x, y int, fg, bg termbox.Attribute, msg string) {
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

	width, _ := termbox.Size()
	for ; x < width; x++ {
		termbox.SetCell(x, y, ' ', fg, bg)
	}
}

func (v *View) movePage(p PagingRequest) {
	_, height := termbox.Size()
	perPage := height - 4

	switch p {
	case ToPrevLine:
		v.currentLine--
	case ToNextLine:
		v.currentLine++
	case ToPrevPage, ToNextPage:
		if p == ToPrevPage {
			v.currentLine -= perPage
		} else {
			v.currentLine += perPage
		}
	}

	if v.currentLine < 1 {
		if v.current != nil {
			// Go to last page, if possible
			v.currentLine = len(v.current)
		} else {
			v.currentLine = 1
		}
	} else if v.current != nil && v.currentLine > len(v.current) {
		v.currentLine = 1
	}
	v.drawScreen(nil)
}

func (v *View) drawScreen(targets []Match) {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	fgAttr := v.config.Style.Basic.fg
	bgAttr := v.config.Style.Basic.bg

	if err := termbox.Clear(fgAttr, bgAttr); err != nil {
		return
	}

	if targets == nil {
		if current := v.Ctx.current; current != nil {
			targets = v.Ctx.current
		} else {
			targets = v.Ctx.lines
		}
	}
	if v.Ctx.currentLine > len(targets) && len(targets) > 0 {
		v.Ctx.currentLine = len(targets)
	}

	width, height := termbox.Size()
	perPage := height - 4

CALCULATE_PAGE:
	currentPage := &v.Ctx.currentPage
	currentPage.index = ((v.Ctx.currentLine - 1) / perPage) + 1
	if currentPage.index <= 0 {
		currentPage.index = 1
	}
	currentPage.offset = (currentPage.index - 1) * perPage
	currentPage.perPage = perPage
	var maxPage int
	if len(targets) == 0 {
		maxPage = 1
	} else {
		maxPage = ((len(targets) + perPage - 1) / perPage)
	}

	if maxPage < currentPage.index {
		if len(targets) == 0 && len(v.Ctx.query) == 0 {
			// wait for targets
			return
		}
		v.Ctx.currentLine = currentPage.offset
		goto CALCULATE_PAGE
	}

	fgAttr = v.config.Style.Query.fg
	bgAttr = v.config.Style.Query.bg

	prompt := "QUERY>"
	promptLen := runewidth.StringWidth(prompt)
	printTB(0, 0, fgAttr, bgAttr, prompt)

	if v.caretPos <= 0 {
		v.caretPos = 0 // sanity
	}
	if v.caretPos > len(v.query) {
		v.caretPos = len(v.query)
	}

	if v.caretPos == len(v.query) {
		// the entire string + the caret after the string
		printTB(promptLen+1, 0, fgAttr, bgAttr, string(v.query))
		termbox.SetCell(promptLen+1+runewidth.StringWidth(string(v.query)), 0, ' ', fgAttr|termbox.AttrReverse, bgAttr|termbox.AttrReverse)
	} else {
		// the caret is in the middle of the string
		prev := 0
		for i, r := range v.query {
			fg := v.config.Style.Query.fg
			bg := v.config.Style.Query.bg
			if i == v.caretPos {
				fg |= termbox.AttrReverse
				bg |= termbox.AttrReverse
			}
			termbox.SetCell(promptLen+1+prev, 0, r, fg, bg)
			prev += runewidth.RuneWidth(r)
		}
	}

	pmsg := fmt.Sprintf("%s [%d/%d]", v.Ctx.Matcher().String(), currentPage.index, maxPage)

	printTB(width-runewidth.StringWidth(pmsg), 0, fgAttr, bgAttr, pmsg)

	for n := 1; n <= perPage; n++ {
		fgAttr = v.config.Style.Basic.fg
		bgAttr = v.config.Style.Basic.bg
		if n+currentPage.offset == v.currentLine {
			fgAttr = v.config.Style.Selected.fg
			bgAttr = v.config.Style.Selected.bg
		} else if v.selection.Has(n + currentPage.offset) || v.RangeSelection().Has(n + currentPage.offset) {
			fgAttr = v.config.Style.SavedSelection.fg
			bgAttr = v.config.Style.SavedSelection.bg
		}

		targetIdx := currentPage.offset + n - 1
		if targetIdx >= len(targets) {
			break
		}

		target := targets[targetIdx]
		line := target.Line()
		matches := target.Indices()
		if matches == nil {
			printTB(0, n, fgAttr, bgAttr, line)
		} else {
			prev := 0
			index := 0
			for _, m := range matches {
				if m[0] > index {
					c := line[index:m[0]]
					printTB(prev, n, fgAttr, bgAttr, c)
					prev += runewidth.StringWidth(c)
					index += len(c)
				}
				c := line[m[0]:m[1]]
				printTB(prev, n, v.config.Style.Matched.fg, bgAttr|v.config.Style.Matched.bg, c)
				prev += runewidth.StringWidth(c)
				index += len(c)
			}

			m := matches[len(matches)-1]
			if m[0] > index {
				printTB(prev, n, v.config.Style.Query.fg, bgAttr|v.config.Style.Query.bg, line[m[0]:m[1]])
			} else if len(line) > m[1] {
				printTB(prev, n, fgAttr, bgAttr, line[m[1]:len(line)])
			}
		}
	}

	if err := termbox.Flush(); err != nil {
		return
	}

	// FIXME
	v.current = targets
}
