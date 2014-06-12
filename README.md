peco
======

Simplistic interfacting filtering tool

Description
===========

peco is based on [percol](https://github.com/mooz/percol). The idea is that percol was darn useful, but I wanted a tool that was a single binary. peco is written in Go, and as of this writing only implements the basic filtering feature (mainly because that's the only thing I use -- you're welcome to send me pull requests to make peco more compatible with percol).

peco can be a great tool to filter stuff like logs, process stats, find files, because unlike grep, you can type as you think and look through the current results.

## Demo

Demos speak more than a thousand words! Here's me looking for a process on my mac. As you can see, you can page through your results, and you can keep changing the query:

![optimized](http://lestrrat.github.io/peco/peco-demo-ps.gif)

Here's me trying to figure out which file to open:

![optimized](http://lestrrat.github.io/peco/peco-demo-filename.gif)

When you combine tools like zsh, peco, and [ghq](https://github.com/motemen/ghq), you can make managing/moving around your huge dev area a piece of cake! (this example doesn't use zsh functions so you can see what I'm doing)

![optimized](http://lestrrat.github.io/peco/peco-demo-ghq.gif)


Features
========

## Incremental search

Search results are filtered as you type. This is great to drill down to the
line you are looking for

Multiple terms turn the query into an "AND" query:

![optimized](https://cloud.githubusercontent.com/assets/554281/3241419/d5fccc5c-f13c-11e3-898b-280a246b083c.gif)

When you find that line that you want, press enter, and the resulting line
is printed to stdout, which allows you to pipe it to other tools

## Case Sensitivity

Case sensitive mode can be toggled on/off 

![optimized](http://lestrrat.github.io/peco/peco-demo-case-sensitive.gif)

## Works on Windows!

I have been told that peco even works on windows :)

Installation
============

### Mac OS X / Homebrew

If you're on OS X and want to use homebrew, use my custom tap:

```
brew tap lestrrat/peco
brew install peco
```

![optimized](http://lestrrat.github.io/peco/peco-demo-homebrew.gif)

### go get

If you want to go the Go way (install in GOPATH/bin) and just want the command:

```
go get github.com/lestrrat/peco/cmd/peco
```

Usage
=====

If you can read Japanese, [here's one cool usage](http://blog.kentarok.org/entry/2014/06/03/135300) using [ghq](https://github.com/motemen/ghq)

Basically, you can define a simple function to easily move around your source code tree:

```zsh
function peco-src () {
    local selected_dir=$(ghq list --full-path | peco --query "$LBUFFER")
    if [ -n "$selected_dir" ]; then
        BUFFER="cd ${selected_dir}"
        zle accept-line
    fi    
    zle clear-screen
}         
zle -N peco-src
```

Or to easily navigate godoc for your local stuff:

```zsh
function peco-godoc() { 
    local selected_dir=$(ghq list --full-path | peco --query "$LBUFFER")
    if [ -n "$selected_dir" ]; then
        BUFFER="godoc ${selected_dir} | less"
        zle accept-line 
    fi 
    zle clear-screen 
}
    
zle -N peco-godoc 
```

Command Line Options
====================

### --help

Display a help message

### --query <query>

Specifies the default query to be used upon startup. This is useful for scripts and functions where you can figure out before hand what the most likely query string is.

### --rcfile <filename>

Pass peco a configuration file, which currently must be a JSON file. If unspecified, it will read ~/.peco/config.json by default (if available)

### --no-ignore-case

By default peco starts in case insensitive mode. When this option is specified, peco will start in case sensitive mode. This can be toggled while peco is still in session.

Configuration File
==================

By default configuration file in ~/.peco/config.json will be searched. You may
also pass an arbitrary filename via the --rcfile option

Currently only keymaps are supported:

```json
{
    "Keymap": {
        "C-p": "peco.SelectPrevious",
        "C-n": "peco.SelectNext"
    }
}
```

## Available keys:

| Name        | Notes |
|-------------|-------|
| C-a ... C-z | Control + whatever character |
| C-1 ... C-8 | Control + 1..8 |
| C-[         ||
| C-]         ||
| C-~         ||
| C-\_        ||
| C-\\\\      | Note that you need to escape the backslash |
| C-/         ||
| Esc         ||
| Tab         ||
| Insert      ||
| Delete      ||
| Home        ||
| End         ||
| Pgup        ||
| Pgdn        ||
| ArrowUp     ||
| ArrowDown   ||
| ArrowLeft   ||
| ArrowRight  ||

## Available actions

| Name | Notes |
|------|-------|
| peco.ForwardChar        | Move caret forward 1 character |
| peco.BackwardChar       | Move caret backward 1 character |
| peco.ForwardWord        | Move caret forward 1 word |
| peco.BackwardWord       | Move caret backward 1 word|
| peco.BeginningOfLine    | Move caret to the beginning of line |
| peco.EndOfLine          | Move caret to the end of line |
| peco.DeleteForwardChar  | Delete one character forward |
| peco.DeleteBackwardChar | Delete one character backward |
| peco.DeleteForwardWord  | Delete one word forward |
| peco.DeleteBackwardWord | Delete one word backward |
| peco.KillEndOfLine      | Delete the characters under the cursor until the end of the line |
| peco.DeleteAll          | Delete all entered characters |
| peco.SelectPreviousPage | Jumps to previous page |
| peco.SelectNextPage     | Jumps to next page|
| peco.SelectPrevious     | Selects previous line |
| peco.SelectNext         | Selects next line |
| peco.RotateMatcher      | Rotate between matchers (by default, ignore-case/no-ignore-case)|
| peco.Finish             | Exits from peco, with success status |
| peco.Cancel             | Exits from peco, with failure status |

Hacking
=======

First, fork this repo, and get your clone locally.

1. Make sure you have [go 1.x](http://golang.org), with GOPATH appropriately set
2. Run `go get github.com/jessevdk/go-flags`
3. Run `go get github.com/mattn/go-runewidth`
4. Run `go get github.com/nsf/termbox-go`

Note that we have a Godeps file in source tree, for now it's just there for a peace of mind. If you already know about [godep](https://github.com/tools/godep), when you may use that instead of steps 2~4

Then from the root of this repository run:

```
go run releng/build.go
```

This will create a `peco` binary in the local directory, with a version string "git@....".
You obviously don't want to install this in the wild.

TODO
====

Test it. In doing so, we may change the repo structure

Implement all(?) of the original percol options

Notes
=====

Obviously, kudos to the original percol: https://github.com/mooz/percol
Much code stolen from https://github.com/mattn/gof
