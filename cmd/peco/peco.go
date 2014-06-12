package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/lestrrat/peco"
	"github.com/nsf/termbox-go"
)

func showHelp() {
	const v = ` 
Usage: percol [options] [FILE]

Options:
  -h, --help            show this help message and exit
  --rcfile=RCFILE       path to the settings file
  --query=QUERY         pre-input query
`
	os.Stderr.Write([]byte(v))
}

type cmdOptions struct {
	Help   bool   `short:"h" long:"help" description:"show this help message and exit"`
	Query  string `long:"query"`
	Rcfile string `long:"rcfile" descriotion:"path to the settings file"`
}

func main() {
	var err error

	opts := &cmdOptions{}
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
			if result[len(result)-1] != '\n' {
				result = result + "\n"
			}
			os.Stdout.WriteString(result)
		}
		// ONLY call exit in this defer
		os.Exit(ctx.ExitStatus)
	}()

	if opts.Rcfile == "" {
		user, err := user.Current()
		if err == nil { // silently ignore failure for user.Current()
			file := filepath.Join(user.HomeDir, ".peco", "config.json")
			_, err := os.Stat(file)
			if err == nil {
				opts.Rcfile = file
			}
		}
	}

	if opts.Rcfile != "" {
		err = ctx.ReadConfig(opts.Rcfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			ctx.ExitStatus = 1
			return
		}
	}

	if err = ctx.ReadBuffer(in); err != nil {
		// Nothing to process, bail out
		fmt.Fprintln(os.Stderr, "You must supply something to work with via filename or stdin")
		ctx.ExitStatus = 1
		return
	}

	// Check TTY
	err = peco.TtyReady()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		ctx.ExitStatus = 1
		return
	}
	defer peco.TtyTerm()

	// Initialize the terminal
	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		ctx.ExitStatus = 1
		return
	}
	defer termbox.Close()

	// View does the drawing
	view := ctx.NewView()
	go view.Loop()

	// Filter runs the query against the current buffer
	filter := ctx.NewFilter()
	go filter.Loop()

	// Input interfaces with the user (accepts key inputs)
	input := ctx.NewInput()
	go input.Loop()

	if len(opts.Query) > 0 {
		ctx.ExecQuery(string(string(opts.Query)))
	} else {
		view.Refresh()
	}

	ctx.WaitDone()
}
