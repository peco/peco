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
  --query=QUERY         pre-input query
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

	var in *os.File

	// receive in from either a file or Stdin
	if len(args) > 0 {
		in, err = os.Open(args[0])
		if err != nil {
			os.Exit(1)
		}
	} else if !peco.IsTty() {
		in = os.Stdin
	}

	ctx := peco.NewCtx()
	defer func() {
		if result := ctx.Result(); result != "" {
			os.Stdout.WriteString(result)
		}
	}()

	if err = ctx.ReadBuffer(in); err != nil {
		// Nothing to process, bail out
		fmt.Fprintln(os.Stderr, "You must supply something to work with via filename or stdin")
		os.Exit(1)
	}

	err = peco.TtyReady()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer termbox.Close()

	view := ctx.NewView()
	filter := ctx.NewFilter()
	input := ctx.NewInput()

	go view.Loop()
	go filter.Loop()
	go input.Loop()

	if len(opts.Query) > 0 {
		ctx.ExecQuery(string(string(opts.Query)))
	} else {
		view.Refresh()
	}

	ctx.WaitDone()
	ctx.PrintResult()
}
