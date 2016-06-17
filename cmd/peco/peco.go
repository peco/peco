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

type canceler interface {
	Canceled() bool
}

func isCanceled(err error) bool {
	ec, ok := err.(canceler)
	if !ok {
		return false
	}
	return ec.Canceled()
}

func _main() int {
	if envvar := os.Getenv("GOMAXPROCS"); envvar == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	ctx := context.Background()

	cli := peco.New()
	if err := cli.Run(ctx); err != nil {
		if !isCanceled(err) {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
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
