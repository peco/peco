// This script forces the user to build 'peco' with an appropriate "version" string
// so that peco --version can change without changing the source code.
//
// The use of +build build in cmd/peco/peco.go forbids the user from building
// the peco binary without knowing what you are doing
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jessevdk/go-flags"
)

type cmdOptions struct {
	Debug   bool   `default:"false" long:"debug" description:"enable debug output"`
	Version string `default:"development-version" long:"version" description:"print the version and exit"`
}

func main() {
	var st int

	defer func() { os.Exit(st) }()

	opts := &cmdOptions{}
	p := flags.NewParser(opts, flags.PrintErrors)
	_, err := p.Parse()
	if err != nil {
		st = 1
		return
	}

	if _, err = os.Stat("peco"); err == nil {
		fmt.Fprintln(os.Stderr, "File 'peco' already exists. removing file...")
		if err = os.Remove("peco"); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove file 'peco'. Bailing out. %s\n", err)
			st = 1
			return
		}
	}

	buildCmd := []string{
		"build",
		"-a",
		"-tags",
		"build",
		"-ldflags",
		fmt.Sprintf("-X main.version %s", opts.Version),
	}
	if opts.Debug {
		buildCmd = append(buildCmd, "-x", "-v")
	}
	buildCmd = append(buildCmd, filepath.Join("github.com", "lestrrat", "peco", "cmd", "peco"))

	cmd := exec.Command("go", buildCmd...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "Executing %v\n", cmd.Args)

	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error while building: %s\n", err)
		st = 1
		return
	}

}