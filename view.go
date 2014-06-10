package percol

import (
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

type View struct {
	*Ctx
}

func (u *View) Loop() {
	u.AddWaitGroup()
	defer u.ReleaseWaitGroup()
	for {
		select {
		case <-u.LoopCh():
			return
		case lines := <-u.DrawCh():
			u.drawScreen(lines)
		}
	}
}

func printTB(x, y int, fg, bg termbox.Attribute, msg string) {
	for len(msg) > 0 {
		c, w := utf8.DecodeRuneInString(msg)
		if c == utf8.RuneError {
			continue
		}
		msg = msg[w:]
		termbox.SetCell(x, y, c, fg, bg)
		x += w
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

	_, height := termbox.Size()
	perPage := height - 4
	currentPage := ((u.Ctx.selectedLine - 1) / perPage) + 1
	if currentPage <= 0 {
		currentPage = 1
	}
	offset := (currentPage - 1) * perPage

	printTB(0, 0, termbox.ColorDefault, termbox.ColorDefault, "QUERY>")
	printTB(8, 0, termbox.ColorDefault, termbox.ColorDefault, string(u.query))

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
			for _, m := range target.matches {
				if m[0] > prev {
					printTB(prev, n, fgAttr, bgAttr, line[prev:m[0]])
					prev += runewidth.StringWidth(line[prev:m[0]])
				}
				printTB(prev, n, fgAttr|termbox.ColorCyan, bgAttr, line[m[0]:m[1]])
				prev += runewidth.StringWidth(line[m[0]:m[1]])
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
