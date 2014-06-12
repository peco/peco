package peco

import (
	"fmt"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

type View struct {
	*Ctx
}

type PagingRequest int

const (
	ToNextLine PagingRequest = iota
	ToNextPage
	ToPrevLine
	ToPrevPage
)

func (u *View) Loop() {
	u.AddWaitGroup()
	defer u.ReleaseWaitGroup()
	for {
		select {
		case <-u.LoopCh():
			return
		case r := <-u.PagingCh():
			u.movePage(r)
		case lines := <-u.DrawCh():
			u.drawScreen(lines)
		}
	}
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
}

func (v *View) movePage(p PagingRequest) {
	_, height := termbox.Size()
	perPage := height - 4

	switch p {
	case ToPrevLine:
		v.selectedLine--
	case ToNextLine:
		v.selectedLine++
	case ToPrevPage, ToNextPage:
		if p == ToPrevPage {
			v.selectedLine -= perPage
		} else {
			v.selectedLine += perPage
		}
	}

	if v.selectedLine < 1 {
		if v.current != nil && len(v.current) > perPage {
			// Go to last page, if possible
			v.selectedLine = len(v.current)
		} else {
			v.selectedLine = 1
		}
	} else if v.current != nil && v.selectedLine > len(v.current) {
		v.selectedLine = 1
	}
}

func (u *View) drawScreen(targets []Match) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	if targets == nil {
		if current := u.Ctx.current; current != nil {
			targets = u.Ctx.current
		} else {
			targets = u.Ctx.lines
		}
	}

	width, height := termbox.Size()
	perPage := height - 4

CALCULATE_PAGE:
	currentPage := ((u.Ctx.selectedLine - 1) / perPage) + 1
	if currentPage <= 0 {
		currentPage = 1
	}
	offset := (currentPage - 1) * perPage
	var maxPage int
	if len(targets) == 0 {
		maxPage = 1
	} else {
		maxPage = ((len(targets) + perPage - 1) / perPage)
	}

	if maxPage < currentPage {
		u.Ctx.selectedLine = offset
		goto CALCULATE_PAGE
	}

	prompt := "QUERY>"
	promptLen := len(prompt)
	printTB(0, 0, termbox.ColorDefault, termbox.ColorDefault, prompt)

	if u.caretPos <= 0 {
		u.caretPos = 0 // sanity
	}
	if u.caretPos > len(u.query) {
		u.caretPos = len(u.query)
	}

	if u.caretPos == len(u.query) {
		// the entire string + the caret after the string
		printTB(promptLen+1, 0, termbox.ColorDefault, termbox.ColorDefault, string(u.query))
		termbox.SetCell(promptLen+1+runewidth.StringWidth(string(u.query)), 0, ' ', termbox.ColorDefault|termbox.AttrReverse, termbox.ColorDefault|termbox.AttrReverse)
	} else {
		// the caret is in the middle of the string
		prev := 0
		for i, r := range u.query {
			fg := termbox.ColorDefault
			bg := termbox.ColorDefault
			if i == u.caretPos {
				fg |= termbox.AttrReverse
				bg |= termbox.AttrReverse
			}
			termbox.SetCell(promptLen+1+prev, 0, r, fg, bg)
			prev += runewidth.RuneWidth(r)
		}
	}

	pmsg := fmt.Sprintf("%s [%d/%d]", u.Ctx.Matcher().String(), currentPage, maxPage)

	printTB(width-runewidth.StringWidth(pmsg), 0, termbox.ColorDefault, termbox.ColorDefault, pmsg)

	for n := 1; n <= perPage; n++ {
		fgAttr := termbox.ColorDefault
		bgAttr := termbox.ColorDefault
		if n == u.selectedLine-offset {
			fgAttr = termbox.AttrUnderline
			bgAttr = termbox.ColorMagenta
		}

		targetIdx := offset + n - 1
		if targetIdx >= len(targets) {
			break
		}
		target := targets[targetIdx]
		line := target.line
		if target.matches == nil {
			printTB(0, n, fgAttr, bgAttr, line)
		} else {
			prev := 0
			index := 0
			for _, m := range target.matches {
				if m[0] > index {
					c := line[index:m[0]]
					printTB(prev, n, fgAttr, bgAttr, c)
					prev += runewidth.StringWidth(c)
					index += len(c)
				}
				c := line[m[0]:m[1]]
				printTB(prev, n, fgAttr|termbox.ColorCyan, bgAttr, c)
				prev += runewidth.StringWidth(c)
				index += len(c)
			}

			m := target.matches[len(target.matches)-1]
			if m[0] > prev {
				printTB(prev, n, fgAttr|termbox.ColorCyan, bgAttr, line[m[0]:m[1]])
			} else if len(line) > m[1] {
				printTB(prev, n, fgAttr, bgAttr, line[m[1]:len(line)])
			}
		}
	}
	termbox.Flush()

	// FIXME
	u.current = targets
}
