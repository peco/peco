package peco

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/peco/peco/config"
)

// CLIOptions holds the command-line flags parsed by go-flags.
type CLIOptions struct {
	OptHelp            bool   `short:"h" long:"help" description:"show this help message and exit"`
	OptQuery           string `long:"query" description:"initial value for query"`
	OptRcfile          string `long:"rcfile" description:"path to the settings file"`
	OptVersion         bool   `long:"version" description:"print the version and exit"`
	OptBufferSize      int    `long:"buffer-size" short:"b" description:"number of lines to keep in search buffer"`
	OptEnableNullSep   bool   `long:"null" description:"expect NUL (\\0) as separator for target/output"`
	OptInitialIndex    int    `long:"initial-index" description:"position of the initial index of the selection (0 base)"`
	OptInitialFilter   string `long:"initial-filter" description:"specify the default filter"`
	OptPrompt          string `long:"prompt" description:"specify the prompt string"`
	OptLayout          string `long:"layout" description:"layout to be used. 'top-down', 'bottom-up', or 'top-down-query-bottom'. default is 'top-down'"`
	OptSelect1         bool   `long:"select-1" description:"select first item and immediately exit if the input contains only 1 item"`
	OptExitZero        bool   `long:"exit-0" description:"exit immediately with status 1 if the input is empty"`
	OptSelectAll       bool   `long:"select-all" description:"select all items and immediately exit"`
	OptOnCancel        string `long:"on-cancel" description:"specify action on user cancel. 'success' or 'error'.\ndefault is 'success'. This may change in future versions"`
	OptSelectionPrefix string `long:"selection-prefix" description:"use a prefix instead of changing line color to indicate currently selected lines.\ndefault is to use colors. This option is experimental"`
	OptExec            string `long:"exec" description:"execute command instead of finishing/terminating peco.\nPlease note that this command will receive selected line(s) from stdin,\nand will be executed via '/bin/sh -c' or 'cmd /c'"`
	OptPrintQuery      bool   `long:"print-query" description:"print out the current query as first line of output"`
	OptColor           string `long:"color" description:"color mode: 'auto' (default, parse ANSI codes) or 'none' (disable)" default:"auto"`
	OptHeight          string `long:"height" description:"display height in lines or percentage (e.g. '10', '50%')"`
}

// parse parses command-line arguments and validates the resulting options.
func (options *CLIOptions) parse(s []string) ([]string, error) {
	p := flags.NewParser(options, flags.PrintErrors)
	args, err := p.ParseArgs(s)
	if err != nil {
		_, _ = os.Stderr.Write(options.help())
		return nil, fmt.Errorf("invalid command line options: %w", err)
	}

	if err := options.Validate(); err != nil {
		return nil, fmt.Errorf("invalid command line arguments: %w", err)
	}

	return args, nil
}

// Validate checks the parsed CLI options for correctness (e.g., layout type).
func (options CLIOptions) Validate() error {
	if options.OptLayout != "" {
		if !config.IsValidLayoutType(options.OptLayout) {
			return errors.New("unknown layout: '" + options.OptLayout + "'")
		}
	}
	return nil
}

// help generates formatted help text from struct field tags.
func (options CLIOptions) help() []byte {
	buf := bytes.Buffer{}

	fmt.Fprintf(&buf, `
Usage: peco [options] [FILE]

Options:
`)

	t := reflect.TypeFor[CLIOptions]()
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag

		var o string
		if s := tag.Get("short"); s != "" {
			o = fmt.Sprintf("-%s, --%s", tag.Get("short"), tag.Get("long"))
		} else {
			o = fmt.Sprintf("--%s", tag.Get("long"))
		}

		// if multiline, we need to indent the proceeding lines
		desc := tag.Get("description")
		if i := strings.Index(desc, "\n"); i >= 0 {
			// first line does not need indenting, so get that out of the way
			var buf bytes.Buffer
			buf.WriteString(desc[:i+1])
			desc = desc[i+1:]
			const indent = "                        "
			for {
				if i = strings.Index(desc, "\n"); i >= 0 {
					buf.WriteString(indent)
					buf.WriteString(desc[:i+1])
					desc = desc[i+1:]
					continue
				}
				break
			}
			if len(desc) > 0 {
				buf.WriteString(indent)
				buf.WriteString(desc)
			}
			desc = buf.String()
		}

		fmt.Fprintf(
			&buf,
			"  %-21s %s\n",
			o,
			desc,
		)
	}

	return buf.Bytes()
}
