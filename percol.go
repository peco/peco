package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/nsf/termbox-go"
)

type Ctx struct {
	result       string
	loopCh       chan struct{}
	mutex        sync.Mutex
	query        []rune
	dirty        bool // true if filtering must be redone
	cursorX      int
	selectedLine int
	lines        []Match
	current      []Match

	wait *sync.WaitGroup
}

type Match struct {
	line    string
	matches [][]int
}

var ctx = Ctx{
	"",
	make(chan struct{}),
	sync.Mutex{},
	[]rune{},
	false,
	0,
	1,
	[]Match{},
	nil,
	&sync.WaitGroup{},
}
var ui = &UI{
	make(chan []Match),
	ctx.wait,
}
var filter = &Filter{
	make(chan string),
	ctx.wait,
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
	Help  bool   `short:"h" long:"help" description:"show this help message and exit"`
	TTY   string `long:"tty" description:"path to the TTY (usually, the value of $TTY)"`
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
		ctx.query = []rune(opts.Query)
	}

	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer termbox.Close()

	termbox.SetInputMode(termbox.InputEsc)

	go ui.Loop()
	go filter.Loop()
	go mainLoop()

	if len(ctx.query) > 0 {
		filter.Execute(string(ctx.query))
	} else {
		ui.Refresh()
	}

	ctx.wait.Wait()
}

func mainLoop() {
	ctx.wait.Add(1)
	defer ctx.wait.Done()

	for {
		select {
		case <-ctx.loopCh: // can only fall here if we closed ctx.loop
			return
		default:
			ev := termbox.PollEvent()
			if ev.Type == termbox.EventError {
				//update = false
			} else if ev.Type == termbox.EventKey {
				handleKeyEvent(ev)
			}
		}
	}
}
