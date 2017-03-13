package main

import (
	"fmt"
	"os"
	"runtime"

	"context"

	pdebug "github.com/lestrrat/go-pdebug"
	"github.com/peco/peco"
	"github.com/peco/peco/internal/util"
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
	if pdebug.Enabled {
		pdebug.DefaultCtx.Writer = os.Stderr
	}
	if envvar := os.Getenv("GOMAXPROCS"); envvar == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli := peco.New()
	if err := cli.Run(ctx); err != nil {
		switch {
		case util.IsCollectResultsError(err):
			cli.PrintResults()
			return 0
		case util.IsIgnorableError(err):
			if st, ok := util.GetExitStatus(err); ok {
				return st
			}
			return 0
		default:
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			return 1
		}

	}

	return 0
}
