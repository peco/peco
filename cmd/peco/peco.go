package main

import (
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
			cli.PrintResults()
			return 0
		case util.IsIgnorableError(err):
			return 0
		default:
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return 1
		}
	}

	return 0
}
