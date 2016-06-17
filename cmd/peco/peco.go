package main

import (
	"bytes"
	"fmt"
	"os"
	"runtime"

	"github.com/peco/peco"
	"golang.org/x/net/context"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stderr, "Error:\n%s", err)
			os.Exit(1)
		}
	}()
	os.Exit(_main())
}
type causer interface {
	Cause() error
}
type ignorable interface {
	Ignorable() bool
}

func isIgnorable(err error) bool {
	for e := err; e != nil; {
		switch e.(type) {
		case ignorable:
			return e.(ignorable).Ignorable()
		case causer:
			e = e.(causer).Cause()
		default:
			return false
		}
	}
	return false
}

func _main() int {
	if envvar := os.Getenv("GOMAXPROCS"); envvar == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	ctx := context.Background()

	cli := peco.New()
	if err := cli.Run(ctx); err != nil {
		if isIgnorable(err) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		return 1
	}

	buf := bytes.Buffer{}
	for line := range cli.ResultCh() {
		buf.WriteString(line.DisplayString())
		buf.WriteByte('\n')
	}
	os.Stdout.Write(buf.Bytes())
	return 0
}
