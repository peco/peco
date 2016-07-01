package main

import (
	"bytes"
	"fmt"
	"os"
	"runtime"

	"github.com/peco/peco"
	"github.com/peco/peco/internal/util"
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

func _main() int {
	if envvar := os.Getenv("GOMAXPROCS"); envvar == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	ctx := context.Background()

	cli := peco.New()
	if err := cli.Run(ctx); err != nil {
		switch {
		case util.IsCollectResultsError(err):
			selection := cli.Selection()
			if selection.Len() == 0 {
				if l, err := cli.CurrentLineBuffer().LineAt(cli.Location().LineNumber()); err == nil {
					selection.Add(l)
				}
			}

			cli.SetResultCh(make(chan peco.Line))
			go cli.CollectResults()
		case util.IsIgnorableError(err):
			return 0
		default:
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return 1
		}
	}

	buf := bytes.Buffer{}
	for line := range cli.ResultCh() {
		buf.WriteString(line.Output())
		buf.WriteByte('\n')
	}
	os.Stdout.Write(buf.Bytes())
	return 0
}
