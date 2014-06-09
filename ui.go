package main

import (
	"sync"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

type UI struct {
	drawCh chan []Match
	wait   *sync.WaitGroup
}

func (u *UI) DrawMatches(m []Match) {
	u.drawCh <- m
}
func (u *UI) Refresh() {
	u.DrawMatches(nil)
}

func (u *UI) Loop() {
	ctx.wait.Add(1)
	defer ctx.wait.Done()
	for {
		select {
		case <-ctx.loopCh:
			return
		case lines := <-u.drawCh:
			if lines != nil {
				ctx.current = lines
			}
			ui.drawScreen()
		}
	}
}

func printTB(x, y int, fg, bg termbox.Attribute, msg string) {
	for len(msg) > 0 {
		c, w := utf8.DecodeRuneInString(msg)
		msg = msg[w:]
		termbox.SetCell(x, y, c, fg, bg)
		x += w
	}
}

func (u *UI) drawScreen() {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	width, height := termbox.Size()
	_ = width
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)

	var targets []Match
	if ctx.current == nil {
		targets = ctx.lines
	} else {
		targets = ctx.current
	}

	printTB(0, 0, termbox.ColorDefault, termbox.ColorDefault, "QUERY>")
	printTB(8, 0, termbox.ColorDefault, termbox.ColorDefault, string(ctx.query))
	for n := 1; n+2 < height; n++ {
		if n-1 >= len(targets) {
			break
		}

		fgAttr := termbox.ColorDefault
		bgAttr := termbox.ColorDefault
		if n == ctx.selectedLine {
			fgAttr = termbox.AttrUnderline
			bgAttr = termbox.ColorMagenta
		}

		target := targets[n-1]
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
}

func handleKeyEvent(ev termbox.Event) {
	switch ev.Key {
	case termbox.KeyEsc:
		termbox.Close()
		close(ctx.loopCh)
		/*
			case termbox.KeyHome, termbox.KeyCtrlA:
				cursor_x = 0
			case termbox.KeyEnd, termbox.KeyCtrlE:
				cursor_x = len(input)
		*/
	case termbox.KeyEnter:
		if len(ctx.current) == 1 {
			ctx.result = ctx.current[0].line
		} else if ctx.selectedLine > 0 && ctx.selectedLine < len(ctx.current) {
			ctx.result = ctx.current[ctx.selectedLine].line
		}
		close(ctx.loopCh)
		/*
			case termbox.KeyArrowLeft:
				if cursor_x > 0 {
					cursor_x--
				}
			case termbox.KeyArrowRight:
				if cursor_x < len([]rune(input)) {
					cursor_x++
				}
		*/
	case termbox.KeyArrowUp, termbox.KeyCtrlK:
		ctx.selectedLine--
		/*
			if cursor_y < len(current)-1 {
				if cursor_y < height-4 {
					cursor_y++
				}
			}
		*/
	case termbox.KeyArrowDown, termbox.KeyCtrlJ:
		ctx.selectedLine++
		/*
				if cursor_y > 0 {
					cursor_y--
				}
			case termbox.KeyCtrlO:
				if cursor_y >= 0 && cursor_y < len(current) {
					*edit = true
					break loop
				}
			case termbox.KeyCtrlI:
				heading = !heading
			case termbox.KeyCtrlL:
				update = true
			case termbox.KeyCtrlU:
				cursor_x = 0
				input = []rune{}
				update = true
		*/
	case termbox.KeyBackspace, termbox.KeyBackspace2:
		if len(ctx.query) >= 0 {
			ctx.query = ctx.query[:len(ctx.query)-1]
			filter.Execute(string(ctx.query))
		}
	default:
		if ev.Key == termbox.KeySpace {
			ev.Ch = ' '
		}

		if ev.Ch > 0 {
			ctx.query = append(ctx.query, ev.Ch)
			filter.Execute(string(ctx.query))
		}
	}
}
