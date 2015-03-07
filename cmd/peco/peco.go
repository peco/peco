package main

import (
	"fmt"
	"os"
	"reflect"
	"runtime"

	"github.com/jessevdk/go-flags"
	"github.com/nsf/termbox-go"
	"github.com/peco/peco"
)

var version = "v0.2.12"

type cmdOptions struct {
	OptHelp           bool   `short:"h" long:"help" description:"show this help message and exit"`
	OptTTY            string `long:"tty" description:"path to the TTY (usually, the value of $TTY)"`
	OptQuery          string `long:"query" description:"initial value for query"`
	OptRcfile         string `long:"rcfile" description:"path to the settings file"`
	OptNoIgnoreCase   bool   `long:"no-ignore-case" description:"start in case-sensitive-mode (DEPRECATED)" default:"false"`
	OptVersion        bool   `long:"version" description:"print the version and exit"`
	OptBufferSize     int    `long:"buffer-size" short:"b" description:"number of lines to keep in search buffer"`
	OptEnableNullSep  bool   `long:"null" description:"expect NUL (\\0) as separator for target/output"`
	OptInitialIndex   int    `long:"initial-index" description:"position of the initial index of the selection (0 base)"`
	OptInitialMatcher string `long:"initial-matcher" description:"specify the default matcher"`
	OptPrompt         string `long:"prompt" description:"specify the prompt string"`
	OptLayout         string `long:"layout" description:"layout to be used 'top-down' (default) or 'bottom-up'" default:"top-down"`
}

func showHelp() {
	// The ONLY reason we're not using go-flags' help option is
	// because I wanted to tweak the format just a bit... but
	// there wasn't an easy way to do so
	os.Stderr.WriteString(`
Usage: peco [options] [FILE]

Options:
`)

	t := reflect.TypeOf(cmdOptions{})
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag

		var o string
		if s := tag.Get("short"); s != "" {
			o = fmt.Sprintf("-%s, --%s", tag.Get("short"), tag.Get("long"))
		} else {
			o = fmt.Sprintf("--%s", tag.Get("long"))
		}

		fmt.Fprintf(
			os.Stderr,
			"  %-21s %s\n",
			o,
			tag.Get("description"),
		)
	}
}

// BufferSize returns the specified buffer size. Fulfills peco.CtxOptions
func (o cmdOptions) BufferSize() int {
	return o.OptBufferSize
}

// EnableNullSep returns tru if --null was specified. Fulfills peco.CtxOptions
func (o cmdOptions) EnableNullSep() bool {
	return o.OptEnableNullSep
}

func (o cmdOptions) InitialIndex() int {
	if o.OptInitialIndex >= 0 {
		return o.OptInitialIndex + 1
	}
	return 1
}

func (o cmdOptions) LayoutType() string {
	return o.OptLayout
}

func main() {
	os.Exit(_main())
}

func _main() (st int) {
	var err error

	if envvar := os.Getenv("GOMAXPROCS"); envvar == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	opts := &cmdOptions{}
	p := flags.NewParser(opts, flags.PrintErrors)
	args, err := p.Parse()
	if err != nil {
		showHelp()
		return 1
	}

	if opts.OptLayout != "" {
		if !peco.IsValidLayoutType(peco.LayoutType(opts.OptLayout)) {
			fmt.Fprintf(os.Stderr, "Unknown layout: '%s'\n", opts.OptLayout)
			return 1
		}
	}

	if opts.OptHelp {
		showHelp()
		return 0
	}

	if opts.OptVersion {
		fmt.Fprintf(os.Stderr, "peco: %s\n", version)
		return 0
	}

	var in *os.File

	// receive in from either a file or Stdin
	switch {
	case len(args) > 0:
		in, err = os.Open(args[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	case !peco.IsTty(os.Stdin.Fd()):
		in = os.Stdin
	default:
		fmt.Fprintln(os.Stderr, "You must supply something to work with via filename or stdin")
		return 1
	}

	ctx := peco.NewCtx(opts)
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stderr, "Error:\n%s", err)
			// XXX does this work?
			st = 1
			return
		}

		ch := ctx.ResultCh()
		if ch == nil {
			return
		}

		for match := range ch {
			line := match.Output()
			if line[len(line)-1] != '\n' {
				line = line + "\n"
			}
			fmt.Fprint(os.Stdout, line)
		}
	}()

	if opts.OptRcfile == "" {
		file, err := peco.LocateRcfile()
		if err == nil {
			opts.OptRcfile = file
		}
	}

	// Default matcher is IgnoreCase
	ctx.MatcherSet.SetCurrentByName(peco.IgnoreCaseMatch)

	if opts.OptRcfile != "" {
		err = ctx.ReadConfig(opts.OptRcfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	if len(opts.OptPrompt) > 0 {
		ctx.SetPrompt(opts.OptPrompt)
	}

	// Deprecated. --no-ignore-case options will be removed in later.
	if opts.OptNoIgnoreCase {
		ctx.MatcherSet.SetCurrentByName(peco.CaseSensitiveMatch)
	}

	if len(opts.OptInitialMatcher) > 0 {
		if !ctx.MatcherSet.SetCurrentByName(opts.OptInitialMatcher) {
			fmt.Fprintf(os.Stderr, "Unknown matcher: '%s'\n", opts.OptInitialMatcher)
			return 1
		}
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
		return 1
	}
	defer peco.TtyTerm()

	err = termbox.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
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
		ctx.SendDraw(nil)
	}

	ctx.WaitDone()

	return ctx.ExitStatus()
}
