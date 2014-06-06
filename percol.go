package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jessevdk/go-flags"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

type Ctx struct {
	result string
	loop bool
	mutex        sync.Mutex
	query        []rune
	dirty        bool // true if filtering must be redone
	cursorX      int
	selectedLine int
	lines        []Match
	current      []Match
}

type Match struct {
	line    string
	matches [][]int
}

var ctx = Ctx{
	"",
	true,
	sync.Mutex{},
	[]rune{},
	false,
	0,
	1,
	[]Match{},
	nil,
}

var timer *time.Timer

func showHelp() {
	const v = ` 
Usage: percol [options] [FILE]

Options:
  -h, --help            show this help message and exit
  --tty=TTY             path to the TTY (usually, the value of $TTY)
  --rcfile=RCFILE       path to the settings file
  --output-encoding=OUTPUT_ENCODING
                        encoding for output
  --input-encoding=INPUT_ENCODING
                        encoding for input and output (default 'utf8')
  --query=QUERY         pre-input query
  --eager               suppress lazy matching (slower, but display correct
                        candidates count)
  --eval=STRING_TO_EVAL
                        eval given string after loading the rc file
  --prompt=PROMPT       specify prompt (percol.view.PROMPT)
  --right-prompt=RIGHT_PROMPT
                        specify right prompt (percol.view.RPROMPT)
  --match-method=MATCH_METHOD
                        specify matching method for query. ` + "`string`" + ` (default)
                        and ` + "`regex`" + ` are currently supported
  --caret-position=CARET
                        position of the caret (default length of the ` + "`query`" + `)
  --initial-index=INDEX
                        position of the initial index of the selection
                        (numeric, "first" or "last")
  --case-sensitive      whether distinguish the case of query or not
  --reverse             whether reverse the order of candidates or not
  --auto-fail           auto fail if no candidates
  --auto-match          auto matching if only one candidate
  --prompt-top          display prompt top of the screen (default)
  --prompt-bottom       display prompt bottom of the screen
  --result-top-down     display results top down (default)
  --result-bottom-up    display results bottom up instead of top down
  --quote               whether quote the output line
  --peep                exit immediately with doing nothing to cache module
                        files and speed up start-up time
`
	os.Stderr.Write([]byte(v))
}

type CmdOptions struct {
	Help bool   `short:"h" long:"help" description:"show this help message and exit"`
	TTY  string `long:"tty" description:"path to the TTY (usually, the value of $TTY)"`
	Query string `long:"query"`
}

func main() {
	var err error

	defer func() {
		if ctx.result != "" {
			os.Stdout.WriteString(ctx.result)
		}
	}()

	opts := &CmdOptions{}
	p := flags.NewParser(opts, flags.PrintErrors)
	args, err := p.Parse() // &opts, os.Args)
	if err != nil {
		panic(err)
		os.Exit(1)
	}

	if opts.Help {
		showHelp()
		os.Exit(1)
	}

	var input *os.File

	// receive input from either a file or Stdin
	if len(args) > 0 {
		input, err = os.Open(args[0])
		if err != nil {
			os.Exit(1)
		}
	} else if !isTty() {
		input = os.Stdin
	}
	rdr := bufio.NewReader(input)
	for {
		line, err := rdr.ReadString('\n')
		if err != nil {
			break
		}

		ctx.lines = append(ctx.lines, Match{line, nil})
	}

	if opts.Query != "" {
		ctx.dirty = true
		ctx.query = []rune(opts.Query)
	}

	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer termbox.Close()

	termbox.SetInputMode(termbox.InputEsc)
	refreshScreen(0)
	mainLoop()
}

func printTB(x, y int, fg, bg termbox.Attribute, msg string) {
	for len(msg) > 0 {
		c, w := utf8.DecodeRuneInString(msg)
		msg = msg[w:]
		termbox.SetCell(x, y, c, fg, bg)
		x += w
	}
}

func filterLines() {
	ctx.current = []Match{}

	re := regexp.MustCompile(regexp.QuoteMeta(string(ctx.query)))
	for _, line := range ctx.lines {
		ms := re.FindAllStringSubmatchIndex(line.line, 1)
		if ms == nil {
			continue
		}
		ctx.current = append(ctx.current, Match{line.line, ms})
	}
	if len(ctx.current) == 0 {
		ctx.current = nil
	}
}

func refreshScreen(delay time.Duration) {
	if timer == nil {
		timer = time.AfterFunc(delay, func() {
			if ctx.dirty {
				filterLines()
			}
			drawScreen()
			ctx.dirty = false
		})
	} else {
		timer.Reset(delay)
	}
}

func drawScreen() {
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

func mainLoop() {
	for ctx.loop {
		ev := termbox.PollEvent()
		if ev.Type == termbox.EventError {
			//update = false
		} else if ev.Type == termbox.EventKey {
			handleKeyEvent(ev)
		}
	}
}

func handleKeyEvent(ev termbox.Event) {
	update := true
	switch ev.Key {
	case termbox.KeyEsc:
		termbox.Close()
		os.Exit(1)
		/*
			case termbox.KeyHome, termbox.KeyCtrlA:
				cursor_x = 0
			case termbox.KeyEnd, termbox.KeyCtrlE:
				cursor_x = len(input)
*/
			case termbox.KeyEnter:
				if len(ctx.current) == 1 {
					ctx.result = ctx.current[0].line
				} else {
					ctx.result = ctx.current[ctx.selectedLine - 1].line
				}
				ctx.loop = false
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
		if len(ctx.query) == 0 {
			update = false
		} else {
			ctx.query = ctx.query[:len(ctx.query)-1]
			ctx.dirty = true
		}
	default:
		if ev.Key == termbox.KeySpace {
			ev.Ch = ' '
		}

		if ev.Ch > 0 {
			ctx.query = append(ctx.query, ev.Ch)
			ctx.dirty = true
		}
	}

	if update {
		refreshScreen(10 * time.Millisecond)
	}
}
