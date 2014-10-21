package main

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strconv"

	"github.com/nsf/termbox-go"
	"github.com/peco/peco"
)

var version = "v0.2.10"

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
	OptLayout         string `long:"layout" description:"layout to be used 'top-down' (default) or 'bottom-up'"`
}

type myFlags struct {
	*flag.FlagSet
}

type myValue struct {
	reflect.Value
}

func (v myValue) IsBoolFlag() bool {
	return v.Value.Type().Kind() == reflect.Bool
}

func (v myValue) Set(x string) error {
	switch v.Value.Type().Kind() {
	case reflect.Slice, reflect.Array:
		v.Value.Set(reflect.Append(v.Value, reflect.ValueOf(x)))
	case reflect.Bool:
		b, err := strconv.ParseBool(x)
		if err != nil {
			return err
		}
		v.Value.Set(reflect.ValueOf(b))
	case reflect.Int:
		i, err := strconv.ParseInt(x, 10, 0)
		if err != nil {
			return err
		}
		v.Value.Set(reflect.ValueOf(int(i)))
	default:
		if !v.Value.CanSet() {
			return fmt.Errorf("cannot set")
		}

		v.Value.Set(reflect.ValueOf(x))
	}
	return nil
}

// Creates a flag.FlagSet struct using informations from cmdOptions struct
func makeFlagSet(o *cmdOptions) *flag.FlagSet {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	ov := reflect.ValueOf(o)
	t := ov.Elem().Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			continue
		}

		tag := f.Tag
		if l := tag.Get("long"); l != "" {
			v := myValue{ov.Elem().Field(i)}
			fs.Var(v, l, "")
			fs.Var(v, "-"+l, "")
		}
	}

	fs.Usage = func() {
		os.Stderr.WriteString(`
Usage: peco [options] [FILE]

Options:
`)
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous {
				continue
			}
			tag := f.Tag
			if tag == "" {
				continue
			}

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

	return fs
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
	var err error
	var st int

	defer func() { os.Exit(st) }()

	if envvar := os.Getenv("GOMAXPROCS"); envvar == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	opts := &cmdOptions{}
	fs := makeFlagSet(opts)
	if err = fs.Parse(os.Args[1:]); err != nil {
		if err != flag.ErrHelp {
			fmt.Fprintf(os.Stderr, "Error parsing command line options: %s\n", err)
			fs.Usage()
		}
		st = 1
		return
	}

	if opts.OptLayout != "" {
		if !peco.IsValidLayoutType(peco.LayoutType(opts.OptLayout)) {
			fmt.Fprintf(os.Stderr, "Unknown layout: '%s'\n", opts.OptLayout)
			st = 1
			return
		}
	}

	if opts.OptVersion {
		fmt.Fprintf(os.Stderr, "peco: %s\n", version)
		return
	}

	var in *os.File

	// receive in from either a file or Stdin
	args := fs.Args()
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
	ctx.MatcherSet.SetCurrentByName(peco.IgnoreCaseMatch)

	if opts.OptRcfile != "" {
		err = ctx.ReadConfig(opts.OptRcfile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			st = 1
			return
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
			st = 1
			return
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

	ctx.WaitDone()

	st = ctx.ExitStatus()
}
