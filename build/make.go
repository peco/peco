package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

/* This script evists because godep deprecated -copy=false, and I really
 * don't agree that importing the actual source code for peco is the
 * correct choice
 */

func main() {
	deps := map[string]string{
		"github.com/jessevdk/go-flags":  "8ec9564882e7923e632f012761c81c46dcf5bec1",
		"github.com/mattn/go-runewidth": "36f63b8223e701c16f36010094fb6e84ffbaf8e0",
		"github.com/nsf/termbox-go":     "e9227d640138066e099db60f3010bd8d55c8da72",
	}

	var pwd string
	var err error
	if pwd, err = os.Getwd(); err != nil {
		panic(err)
	}

	for dir, hash := range deps {
		repo := repoURL(dir)
		dir = filepath.Join("src", dir)
		if _, err = os.Stat(dir); err != nil {
			if err = exec.Command("git", "clone", repo, dir).Run(); err != nil {
				panic(err)
			}
		}

		if err = os.Chdir(dir); err != nil {
			panic(err)
		}

		if err = exec.Command("git", "reset", "--hard").Run(); err != nil {
			panic(err)
		}

		if err = exec.Command("git", "checkout", "master").Run(); err != nil {
			panic(err)
		}

		if err = exec.Command("git", "pull").Run(); err != nil {
			panic(err)
		}

		if err = exec.Command("git", "checkout", hash).Run(); err != nil {
			panic(err)
		}

		if err = os.Chdir(pwd); err != nil {
			panic(err)
		}
	}

	// Link src/github.com/peco/peco to updir
	pecodir := filepath.Join("src", "github.com", "peco", "peco")
	parent := filepath.Dir(pecodir)
	if _, err = os.Stat(parent); err != nil {
		if err = os.MkdirAll(parent, 0777); err != nil {
			panic(err)
		}
	}

	updir, err := filepath.Abs(filepath.Join(pwd, ".."))
	if err != nil {
		panic(err)
	}

	if _, err := os.Stat(pecodir); err != nil {
		if err = os.Symlink(updir, pecodir); err != nil {
			panic(err)
		}
	}

	if err = os.Chdir(pecodir); err != nil {
		panic(err)
	}

	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		gopath = strings.Join([]string{pwd, gopath}, ":")
	}
	os.Setenv("GOPATH", gopath)

	goxcArgs := []string {
		"-tasks", "xc archive",
		"-bc", "linux windows darwin",
		"-d", os.Args[1],
		"-resources-include", "README*",
		"-resources-include", "Changes",
		"-main-dirs-exclude", "_demos,examples,build",
	}
	if err = run("goxc", goxcArgs...); err != nil {
		panic(err)
	}
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	for _, line := range strings.SplitN(string(out), "\n", -1) {
		log.Print(line)
	}
	return err
}

func repoURL(spec string) string {
	return "https://" + spec + ".git"
}