package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jessevdk/go-flags"
)

type cmdOptions struct {
	Version string `required:"true" long:"version" description:"print the version and exit"`
}

var versionRe = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
var yesRe = regexp.MustCompile(`(?i)^y(?:es)?$`)

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

	if !versionRe.MatchString(opts.Version) {
		fmt.Fprintf(os.Stderr, "Version strings must be in the form 'vX.Y.Z', not '%s'\n", opts.Version)
		st = 1
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while attempting to get current working directory: %s", err)
		st = 1
		return
	}

	if err = os.Remove(filepath.Join(cwd, "peco")); err != nil {
		fmt.Fprintf(os.Stderr, "Error while cleaning up for 'peco' binary: %s", err)
		st = 1
		return
	}

	var out *bytes.Buffer
	var cmd *exec.Cmd

	// Check if this working tree is dirty
	out = &bytes.Buffer{}
	cmd = exec.Command("git", "diff", "--shortstat")
	cmd.Stdout = out

	fmt.Fprintf(os.Stderr, "Executing %v\n", cmd.Args)
	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error while checking for dirty files: %s\n", err)
		return
	}

	if out.String() != "" {
		fmt.Fprintln(os.Stderr, "Working tree is ditry. Please commit all changes first")
		st = 1
		return
	}

	// Check if we have untracked files
	out = &bytes.Buffer{}
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Stdout = out

	fmt.Fprintf(os.Stderr, "Executing %v\n", cmd.Args)
	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error while checking for dirty files: %s\n", err)
		return
	}

	if strings.Contains(out.String(), "??") {
		fmt.Fprintln(os.Stderr, "Working tree contains untracked files. Please commit or remove them first")
		st = 1
		return
	}

	cmd = exec.Command(
		"go",
		"run",
		filepath.Join("releng", "build.go"),
		"--version",
		opts.Version,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "Executing %v\n", cmd.Args)

	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error while building: %s\n", err)
		st = 1
		return
	}

	cmd = exec.Command(filepath.Join(cwd, "peco"), "--version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "Executing %v\n", cmd.Args)

	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error while checking for version: %s\n", err)
		st = 1
		return
	}

	if err = os.Remove(filepath.Join(cwd, "peco")); err != nil {
		fmt.Fprintf(os.Stderr, "Error while cleaning up for 'peco' binary: %s", err)
		st = 1
		return
	}

	fmt.Fprint(os.Stdout, "Really release this (i.e. git tag, git push)? [y/N] ")

	var input string
	fmt.Scanf("%s", &input)
	if !yesRe.MatchString(input) {
		return
	}

	// Okay, let's go!
	cmd = exec.Command("git", "tag", opts.Version)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "Executing %v\n", cmd.Args)

	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error while tagging git repository: %s\n", err)
		st = 1
		return
	}

	// git push it
	cmd = exec.Command("git", "push", "--tags", "origin", "master")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Fprintf(os.Stderr, "Executing %v\n", cmd.Args)

	if err = cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error while pushing git repository: %s\n", err)
		st = 1
		return
	}
}
