package peco

import (
	"fmt"
	"os"
	"reflect"
	"runtime"

	"github.com/jessevdk/go-flags"
	"github.com/nsf/termbox-go"
)

type CLIOptions struct {
	OptHelp           bool   `short:"h" long:"help" description:"show this help message and exit"`
	OptTTY            string `long:"tty" description:"path to the TTY (usually, the value of $TTY)"`
	OptQuery          string `long:"query" description:"initial value for query"`
	OptRcfile         string `long:"rcfile" description:"path to the settings file"`
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

	t := reflect.TypeOf(CLIOptions{})
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

// BufferSize returns the specified buffer size. Fulfills CtxOptions
func (o CLIOptions) BufferSize() int {
	return o.OptBufferSize
}

// EnableNullSep returns tru if --null was specified. Fulfills CtxOptions
func (o CLIOptions) EnableNullSep() bool {
	return o.OptEnableNullSep
}

func (o CLIOptions) InitialIndex() int {
	if o.OptInitialIndex >= 0 {
		return o.OptInitialIndex + 1
	}
	return 1
}

func (o CLIOptions) LayoutType() string {
	return o.OptLayout
}

type CLI struct {
}

func (cli *CLI) parseOptions() (*CLIOptions, []string, error) {
	opts := &CLIOptions{}
	p := flags.NewParser(opts, flags.PrintErrors)
	args, err := p.Parse()
	if err != nil {
		showHelp()
		return nil, nil, err
	}

	if opts.OptLayout != "" {
		if !IsValidLayoutType(LayoutType(opts.OptLayout)) {
			return nil, nil, fmt.Errorf("unknown layout: '%s'\n", opts.OptLayout)
		}
	}

	return opts, args, nil
}

func (cli *CLI) Run() error {
	opts, args, err := cli.parseOptions()
	if err != nil {
		return err
	}

	if opts.OptHelp {
		showHelp()
		return nil
	}

	if opts.OptVersion {
		fmt.Fprintf(os.Stderr, "peco: %s\n", version)
		return nil
	}

	var in *os.File

	// receive in from either a file or Stdin
	switch {
	case len(args) > 0:
		in, err = os.Open(args[0])
		if err != nil {
			return err
		}
	case !IsTty(os.Stdin.Fd()):
		in = os.Stdin
	default:
		return fmt.Errorf("You must supply something to work with via filename or stdin")
	}

	ctx := NewCtx(opts)
	defer func() {
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
		file, err := LocateRcfile()
		if err == nil {
			opts.OptRcfile = file
		}
	}

	// Default matcher is IgnoreCase
	ctx.SetCurrentFilterByName(IgnoreCaseMatch)

	if opts.OptRcfile != "" {
		err = ctx.ReadConfig(opts.OptRcfile)
		if err != nil {
			return err
		}
	}

	if len(opts.OptPrompt) > 0 {
		ctx.SetPrompt(opts.OptPrompt)
	}

	if len(opts.OptInitialMatcher) > 0 {
		if err := ctx.SetCurrentFilterByName(opts.OptInitialMatcher); err != nil {
			return fmt.Errorf("unknown matcher: '%s'\n", opts.OptInitialMatcher)
		}
	}

	// Try waiting for something available in the source stream
	// before doing any terminal initialization (also done by termbox)
	reader := ctx.NewBufferReader(in)
	ctx.AddWaitGroup(1)
	go reader.Loop()

	// This channel blocks until we receive something from `in`
	<-reader.InputReadyCh()

	err = TtyReady()
	if err != nil {
		return err
	}
	defer TtyTerm()

	err = termbox.Init()
	if err != nil {
		return err
	}
	defer termbox.Close()

	// Windows handle Esc/Alt self
	if runtime.GOOS == "windows" {
		termbox.SetInputMode(termbox.InputEsc | termbox.InputAlt)
	}

	ctx.startInput()
	view := ctx.NewView()
	filter := ctx.NewFilter()
	sig := ctx.NewSignalHandler()

	loopers := []interface {
		Loop()
	}{
		view,
		filter,
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

	return ctx.Error()
}
