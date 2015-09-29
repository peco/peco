package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

/* This script exists because godep deprecated -copy=false, and I really
 * don't agree that importing the actual source code for peco is the
 * correct choice
 *
 * It's used by my own build script to release files to GitHub.
 * You (the contributor) do not need to use it, but if you need a
 * particular revision of the dependent packages, make sure to update
 * the SHA1 below.
 */

var pwd string
var deps = map[string]string{
	"github.com/jessevdk/go-flags":  "8ec9564882e7923e632f012761c81c46dcf5bec1",
	"github.com/mattn/go-runewidth": "58a0da4ed7b321c9b5dfeffb7e03ee188fae1c60",
	"github.com/nsf/termbox-go":     "10f14d7408b64a659b7c694a771f5006952d336c",
	"github.com/google/btree":       "0c05920fc3d98100a5e3f7fd339865a6e2aaa671",
}

func init() {
	var err error
	if pwd, err = os.Getwd(); err != nil {
		panic(err)
	}
}

func main() {
	switch os.Args[1] {
	case "deps":
		setupDeps()
	case "build":
		setupDeps()
		buildBinaries()
	default:
		panic("Unknown action: " + os.Args[1])
	}
}

func getBuildDir() string {
	buildDir := ".build"
	if dir := os.Getenv("PECO_BUILD_DIR"); dir != "" {
		buildDir = dir
	}

	d, err := filepath.Abs(buildDir)
	if err != nil {
		panic(err)
	}
	buildDir = d

	for _, subdir := range []string{"bin", "pkg", "src", "artifacts"} {
		dir := filepath.Join(buildDir, subdir)

		// Make sure this directory exists to avoid errors...
		if _, err := os.Stat(dir); err != nil {
			if err := os.MkdirAll(dir, 0777); err != nil {
				panic(err)
			}
		}
	}

	return buildDir
}

func setupDeps() {
	var err error

	buildDir := getBuildDir()
	for dir, hash := range deps {
		repo := repoURL(dir)
		dir = filepath.Join(buildDir, "src", dir)
		if _, err = os.Stat(dir); err != nil {
			if err = run("git", "clone", repo, dir); err != nil {
				panic(err)
			}
		}

		if err = os.Chdir(dir); err != nil {
			panic(err)
		}

		if err = run("git", "reset", "--hard"); err != nil {
			panic(err)
		}

		if err = run("git", "checkout", "master"); err != nil {
			panic(err)
		}

		if err = run("git", "pull"); err != nil {
			panic(err)
		}

		if err = run("git", "checkout", hash); err != nil {
			panic(err)
		}

		if err = os.Chdir(pwd); err != nil {
			panic(err)
		}
	}

	log.Println("dependencies have been checked out to under %s", buildDir)
}

func buildBinaries() {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	buildDir := getBuildDir()
	os.Setenv("GOPATH", buildDir)

	// Make a symlink to current directory so that it's in GOPATH
	linkDest := filepath.Join(buildDir, "src", "github.com", "peco", "peco")
	linkDestDir := filepath.Dir(linkDest)
	if _, err := os.Stat(linkDestDir); err != nil {
		if err := os.MkdirAll(linkDestDir, 0777); err != nil {
			panic(err)
		}
	}

	if _, err := os.Stat(linkDest); err != nil {
		log.Printf("Creating symlink from '%s' to '%s'", wd, linkDest)
		if err := os.Symlink(wd, linkDest); err != nil {
			panic(err)
		}
	}

	for _, osname := range []string{"darwin", "linux", "windows"} {
		for _, arch := range []string{"amd64", "386"} {
			buildBinaryFor(osname, arch)
		}
	}

	buildBinaryFor("linux", "arm")
}

func buildBinaryFor(osname, arch string) {
	os.Setenv("GOOS", osname)
	os.Setenv("GOARCH", arch)

	log.Printf("Building for %s/%s", osname, arch)

	name := fmt.Sprintf("peco_%s_%s", osname, arch)

	run("go", "build", "-o",
		filepath.Join(name, "peco"),
		filepath.Join("cmd", "peco", "peco.go"),
	)

	run("cp", "Changes", filepath.Join(name, "Changes"))
	run("cp", "README.md", filepath.Join(name, "README.md"))

	var file string
	if osname == "linux" {
		file = fmt.Sprintf("%s.tar.gz", name)
		run("tar", "cvf", file, name)
	} else {
		file = fmt.Sprintf("%s.zip", name)
		run("zip", "-r", file, name)
	}

	os.RemoveAll(name)
	run("mv", file, filepath.Join(getBuildDir(), "artifacts"))
}

func run(name string, args ...string) error {
	splat := []string{name}
	splat = append(splat, args...)
	log.Printf("---> Running %v...\n", splat)

	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	for _, line := range strings.Split(string(out), "\n") {
		log.Print(line)
	}
	log.Println("")
	log.Println("<---DONE")
	return err
}

func repoURL(spec string) string {
	return "https://" + spec + ".git"
}
