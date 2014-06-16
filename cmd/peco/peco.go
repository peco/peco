package main

import (
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"

	"github.com/jessevdk/go-flags"
	"github.com/lestrrat/peco"
	"github.com/nsf/termbox-go"
)

var version = "v0.1.1"

func showHelp() {
	const v = ` 
Usage: peco [options] [FILE]

Options:
  -h, --help            show this help message and exit
  --version             print the version and exit
  --rcfile=RCFILE       path to the settings file
  --query=QUERY         pre-input query
  --no-ignore-case      start in case-sensitive mode
`
	os.Stderr.Write([]byte(v))
}

type CmdOptions struct {
	Help         bool   `short:"h" long:"help" description:"show this help message and exit"`
	TTY          string `long:"tty" description:"path to the TTY (usually, the value of $TTY)"`
	Query        string `long:"query"`
	Rcfile       string `long:"rcfile" descriotion:"path to the settings file"`
	NoIgnoreCase bool   `long:"no-ignore-case" description:"start in case-sensitive-mode" default:"false"`
	Version      bool   `long:"version" description:"print the version and exit"`
}

func main() {
	var err error
	var st int

	defer func() { os.Exit(st) }()

	opts := &CmdOptions{}
	p := flags.NewParser(opts, flags.PrintErrors)
	args, err := p.Parse()
	if err != nil {
		showHelp()
		st = 1
		return
	}

	if opts.Help {
		showHelp()
		return
	}

	if opts.Version {
		fmt.Fprintf(os.Stderr, "peco: %s\n", version)
		return
	}

	var in *os.File

	// receive in from either a file or Stdin
	if len(args) > 0 {
		in, err = os.Open(args[0])
		if err != nil {
			st = 1
			return
		}
	} else if !peco.IsTty() {
		in = os.Stdin
	}

	ctx := peco.NewCtx()
	defer func() {
		if err := recover(); err != nil {
			st = 1
			fmt.Fprintf(os.Stderr, "Error:\n%s", err)
		}

		if result := ctx.Result(); result != "" {
			if result[len(result)-1] != '\n' {
				result = result + "\n"
			}
			os.Stdout.WriteString(result)
		}
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

	// Default matcher is IgnoreCase
	ctx.SetCurrentMatcher(peco.IgnoreCaseMatch)

	if opts.Rcfile != "" {
		err = ctx.ReadConfig(opts.Rcfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			st = 1
			return
		}
	}

	if opts.NoIgnoreCase {
		ctx.SetCurrentMatcher(peco.CaseSensitiveMatch)
	}

	if err = ctx.ReadBuffer(in); err != nil {
		// Nothing to process, bail out
		fmt.Fprintln(os.Stderr, "You must supply something to work with via filename or stdin")
		st = 1
		return
	}

	err = peco.TtyReady()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		st = 1
		return
	}
	defer peco.TtyTerm()

	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		st = 1
		return
	}
	defer termbox.Close()

	view := ctx.NewView()
	filter := ctx.NewFilter()
	input := ctx.NewInput()

	// AddWaitGroup must be called in this main thread
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ctx.AddWaitGroup(4)
	go view.Loop()
	go filter.Loop()
	go input.Loop()
	go ctx.SignalHandlerLoop(sigCh)

	if len(opts.Query) > 0 {
		ctx.SetQuery([]rune(opts.Query))
		ctx.ExecQuery(opts.Query)
	} else {
		view.Refresh()
	}

	ctx.WaitDone()
}
