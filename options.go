package peco

import (
	"bytes"
	"fmt"
	"os"
	"reflect"

	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
)

// BufferSize returns the specified buffer size. Fulfills CtxOptions
func (o CLIOptions) BufferSize() int {
	return o.OptBufferSize
}

// EnableNullSep returns true if --null was specified. Fulfills CtxOptions
func (o CLIOptions) EnableNullSep() bool {
	return o.OptEnableNullSep
}

func (o CLIOptions) InitialIndex() int {
	return o.OptInitialIndex
}

func (o CLIOptions) LayoutType() string {
	return o.OptLayout
}
func (options *CLIOptions) parse(s []string) ([]string, error) {
	p := flags.NewParser(options, flags.PrintErrors)
	args, err := p.ParseArgs(s)
	if err != nil {
		os.Stderr.Write(options.help())
		return nil, errors.Wrap(err, "invalid command line options")
	}

	if err := options.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid command line arguments")
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

	t := reflect.TypeOf(options)
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag

		var o string
		if s := tag.Get("short"); s != "" {
			o = fmt.Sprintf("-%s, --%s", tag.Get("short"), tag.Get("long"))
		} else {
			o = fmt.Sprintf("--%s", tag.Get("long"))
		}

		fmt.Fprintf(
			&buf,
			"  %-21s %s\n",
			o,
			tag.Get("description"),
		)
	}

	return buf.Bytes()
}
