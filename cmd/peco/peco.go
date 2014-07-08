package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/jessevdk/go-flags"
	"github.com/nsf/termbox-go"
	"github.com/peco/peco"
)

var version = "v0.2.0"

func showHelp() {
	const v = ` 
Usage: peco [options] [FILE]

Options:
  -h, --help            show this help message and exit
  --version             print the version and exit
  --rcfile=RCFILE       path to the settings file
  --query=QUERY         pre-input query
  --no-ignore-case      start in case-sensitive mode
  -b, --buffer-size     number of lines to keep in search buffer
  --null                expect NUL (\0) as separator for target/output (EXPERIMENTAL)
  --initial-index       position of the initial index of the selection (0 base)
  --prompt              specify prompt
  --prompt-bottom       display prompt bottom of the screen
  --result-bottom-up    display results bottom up instead of top down
`
	os.Stderr.Write([]byte(v))
}

type cmdOptions struct {
	OptHelp           bool   `short:"h" long:"help" description:"show this help message and exit"`
	OptTTY            string `long:"tty" description:"path to the TTY (usually, the value of $TTY)"`
	OptQuery          string `long:"query"`
	OptRcfile         string `long:"rcfile" descriotion:"path to the settings file"`
	OptNoIgnoreCase   bool   `long:"no-ignore-case" description:"start in case-sensitive-mode" default:"false"`
	OptVersion        bool   `long:"version" description:"print the version and exit"`
	OptBufferSize     int    `long:"buffer-size" short:"b" description:"number of lines to keep in search buffer"`
	OptEnableNullSep  bool   `long:"null" description:"expect NUL (\\0) as separator for target/output"`
	OptInitialIndex   int    `long:"initial-index" description:"position of the initial index of the selection (0 base)"`
	OptPrompt         string `long:"prompt"`
	OptPromptBottom   bool   `long:"prompt-bottom" description:"display prompt bottom of the screen"`
	OptResultBottomUp bool   `long:"result-bottom-up" description:"display results bottom up instead of top down"`
}

// BufferSize returns the specified buffer size. Fulfills peco.CtxOptions
func (o cmdOptions) BufferSize() int {
	return o.OptBufferSize
}

// EnableNullSep returns tru if --null was specified. Fulfills peco.CtxOptions
func (o cmdOptions) EnableNullSep() bool {
	return o.OptEnableNullSep
}

func (o cmdOptions) PromptBottom() bool {
	return o.OptPromptBottom
}

func (o cmdOptions) ResultBottomUp() bool {
	return o.OptResultBottomUp
}

func (o cmdOptions) InitialIndex() int {
	if o.OptInitialIndex >= 0 {
		return o.OptInitialIndex + 1
	} else {
		return 1
	}
}

func main() {
	var err error
	var st int

	defer func() { os.Exit(st) }()

	if envvar := os.Getenv("GOMAXPROCS"); envvar == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	opts := &cmdOptions{}
	p := flags.NewParser(opts, flags.PrintErrors)
	args, err := p.Parse()
	if err != nil {
		showHelp()
		st = 1
		return
	}

	if opts.OptHelp {
		showHelp()
		return
	}

	if opts.OptVersion {
		fmt.Fprintf(os.Stderr, "peco: %s\n", version)
		return
	}

	var in *os.File

	// receive in from either a file or Stdin
	switch {
	case len(args) > 0:
		in, err = os.Open(args[0])
		if err != nil {
			st = 1
			fmt.Fprintln(os.Stderr, err)
			return
		}
	case !peco.IsTty(os.Stdin.Fd()):
		in = os.Stdin
	default:
		fmt.Fprintln(os.Stderr, "You must supply something to work with via filename or stdin")
		st = 1
		return
	}

	ctx := peco.NewCtx(opts)
	defer func() {
		if err := recover(); err != nil {
			st = 1
			fmt.Fprintf(os.Stderr, "Error:\n%s", err)
		}

		if result := ctx.Result(); result != nil {
			for _, match := range result {
				line := match.Output()
				if line[len(line)-1] != '\n' {
					line = line + "\n"
				}
				fmt.Fprint(os.Stdout, line)
			}
		}
	}()

	if opts.OptRcfile == "" {
		file, err := peco.LocateRcfile()
		if err == nil {
			opts.OptRcfile = file
		}
	}

	// Default matcher is IgnoreCase
	ctx.SetCurrentMatcher(peco.IgnoreCaseMatch)

	if opts.OptRcfile != "" {
		err = ctx.ReadConfig(opts.OptRcfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			st = 1
			return
		}
	}

	if opts.OptNoIgnoreCase {
		ctx.SetCurrentMatcher(peco.CaseSensitiveMatch)
	}

	// Try waiting for something available in the source stream
	// before doing any terminal initialization (also done by termbox)
	reader := ctx.NewBufferReader(in)
	ctx.AddWaitGroup(1)
	go reader.Loop()

	// This channel blocks until we receive something from `in`
	<-reader.InputReadyCh()

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

	// Windows handle Esc/Alt self
	if runtime.GOOS == "windows" {
		termbox.SetInputMode(termbox.InputEsc | termbox.InputAlt)
	}

	view := ctx.NewView()
	filter := ctx.NewFilter()
	input := ctx.NewInput()
	sig := ctx.NewSignalHandler()

	loopers := []interface {
		Loop()
	}{
		view,
		filter,
		input,
		sig,
	}
	for _, looper := range loopers {
		ctx.AddWaitGroup(1)
		go looper.Loop()
	}

	if len(opts.OptQuery) > 0 {
		ctx.SetQuery([]rune(opts.OptQuery))
		ctx.ExecQuery()
	} else {
		view.Refresh()
	}

	if len(opts.OptPrompt) > 0 {
		ctx.SetPrompt([]rune(opts.OptPrompt))
	}

	ctx.WaitDone()

	st = ctx.ExitStatus
}
