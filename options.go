package peco

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/jessevdk/go-flags"
)

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

func (options CLIOptions) Validate() error {
	if options.OptLayout != "" {
		if !IsValidLayoutType(LayoutType(options.OptLayout)) {
			return errors.New("unknown layout: '" + options.OptLayout + "'")
		}
	}
	return nil
}

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
