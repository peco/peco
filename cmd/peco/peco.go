package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/peco/peco"
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

	cli := peco.CLI{}
	if err := cli.Run(); err != nil {
		if err != peco.ErrUserCanceled {
			fmt.Fprintf(os.Stderr, "Error: %s", err)
		}
		return 1
	}
	return 0
}
