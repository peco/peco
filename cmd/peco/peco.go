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

type CmdOptions struct {
	Help   bool   `short:"h" long:"help" description:"show this help message and exit"`
	TTY    string `long:"tty" description:"path to the TTY (usually, the value of $TTY)"`
	Query  string `long:"query"`
	Rcfile string `long:"rcfile" descriotion:"path to the settings file"`
}

func main() {
	var err error

	opts := &CmdOptions{}
	p := flags.NewParser(opts, flags.PrintErrors)
	args, err := p.Parse() // &opts, os.Args)
	if err != nil {
		showHelp()
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

	err = peco.TtyReady()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		ctx.ExitStatus = 1
		return
	}
	defer peco.TtyTerm()

	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		ctx.ExitStatus = 1
		return
	}
	defer termbox.Close()

	view := ctx.NewView()
	filter := ctx.NewFilter()
	input := ctx.NewInput()

	go view.Loop()
	go filter.Loop()
	go input.Loop()

	if len(opts.Query) > 0 {
		ctx.SetQuery([]rune(opts.Query))
		ctx.ExecQuery(opts.Query)
	} else {
		view.Refresh()
	}

	ctx.WaitDone()
}
