package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/lestrrat/go-percol"
	"github.com/nsf/termbox-go"
)

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
	} else if !percol.IsTty() {
		input = os.Stdin
	}

	ctx := percol.NewCtx()
	defer func() {
		if result := ctx.Result(); result != "" {
			os.Stdout.WriteString(result)
		}
	}()

	ctx.ReadBuffer(input)

	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer termbox.Close()
	termbox.SetInputMode(termbox.InputEsc)

	ui := ctx.NewUI()
	filter := ctx.NewFilter()

	go ui.Loop()
	go filter.Loop()
	go ctx.Loop()

	if len(opts.Query) > 0 {
		ctx.ExecQuery(string(string(opts.Query)))
	} else {
		ui.Refresh()
	}

	ctx.WaitDone()
	ctx.PrintResult()
}
