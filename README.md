# peco

Simplistic interactive filtering tool

*NOTE*: If you are viewing this on Github, this document refers to the state of `peco` in whatever current branch you are viewing, _not_ necessarily the state of a currently released version. Please make sure to checkout the [Changes](./Changes) file for features and changes.

This README is long and comprehensive. Use the [Table of Contents](#table-of-contents) to navigate to the section that interests you. It has been placed at the bottom of the README file because of its length.


# Description

`peco` (pronounced *peh-koh*) is based on a python tool, [percol](https://github.com/mooz/percol). `percol` was darn useful, but I wanted a tool that was a single binary, and forget about python. `peco` is written in Go, and therefore you can just grab [the binary releases](https://github.com/peco/peco/releases) and drop it in your $PATH.

`peco` can be a great tool to filter stuff like logs, process stats, find files, because unlike grep, you can type as you think and look through the current results.

For basic usage, continue down below. For more cool elaborate usage samples, [please see the wiki](https://github.com/peco/peco/wiki/Sample-Usage), and if you have any other tricks you want to share, please add to it!

## Demo

Demos speak more than a thousand words! Here's me looking for a process on my mac. As you can see, you can page through your results, and you can keep changing the query:

![Executed `ps -ef | peco`, then the query `root` was typed. This shows all lines containing the word root](http://peco.github.io/images/peco-demo-ps.gif)

Here's me trying to figure out which file to open:

![Executed `find . -name '*.go' | peco` (within camlistore repository), then the query `camget` was typed. This shows all lines including the word `camget`](http://peco.github.io/images/peco-demo-filename.gif)

When you combine tools like zsh, peco, and [ghq](https://github.com/motemen/ghq), you can make managing/moving around your huge dev area a piece of cake! (this example doesn't use zsh functions so you can see what I'm doing)

![Executed `cd $(ghq list --full-path | peco --query peco)` to show all repositories containing the word `peco`, then to change directories into the one selected](http://peco.github.io/images/peco-demo-ghq.gif)


# Features

## Incremental Search

Search results are filtered as you type. This is great to drill down to the
line you are looking for

Multiple terms turn the query into an "AND" query:

![Executed `ps aux | peco`, then the query `root app` was typed. This shows all lines containing both `root` and `app`](http://peco.github.io/images/peco-demo-multiple-queries.gif)

When you find that line that you want, press enter, and the resulting line
is printed to stdout, which allows you to pipe it to other tools

## Select Multiple Lines

You can select multiple lines! 

![Executed `ls -l | peco`, then used peco.ToggleSelection to select multiple lines](http://peco.github.io/images/peco-demo-multiple-selection.gif)

## Select Range Of Lines

Not only can you select multiple lines one by one, you can select a range of lines (Note: The ToggleRangeMode action is not enabled by default. You need to put a custom key binding in your config file)

![Executed `ps -ef | peco`, then used peco.ToggleRangeMode to select a range of lines](http://peco.github.io/images/peco-demo-range-mode.gif)

## Select Filters

Different types of filters are available. Default is case-insensitive filter, so lines with any case will match. You can toggle between IgnoreCase, CaseSensitive, SmartCase RegExp and Fuzzy filters. 

The SmartCase filter uses case-*insensitive* matching when all of the queries are lower case, and case-*sensitive* matching otherwise.

The RegExp filter allows you to use any valid regular expression to match lines

The Fuzzy filter allows you to find matches using partial patterns. For example, when searching for `ALongString`, you can enable the Fuzzy filter and search `ALS` to find it. The Fuzzy filter uses smart case search like the SmartCase filter.

![Executed `ps aux | peco`, then typed `google`, which matches the Chrome.app under IgnoreCase filter type. When you change it to Regexp filter, this is no longer the case. But you can type `(?i)google` instead to toggle case-insensitive mode](http://peco.github.io/images/peco-demo-matcher.gif)

## Selectable Layout

As of v0.2.5, if you would rather not move your eyes off of the bottom of the screen, you can change the screen layout by either providing the `--layout=bottom-up` command line option, or set the `Layout` variable in your configuration file

![Executed `ps -ef | peco --layout=bottom-up` to toggle inverted layout mode](http://peco.github.io/images/peco-demo-layout-bottom-up.gif)

## Works on Windows!

I have been told that peco even works on windows :) Look ma! I'm not lying!

![Showing peco running on Windows cmd.exe](https://gist.githubusercontent.com/taichi/26814518d8b00352693b/raw/b7745987de32dbf068e81a8308c0c5ed38138649/peco.gif)

# Installation

### Just want the binary?

Go to the [releases page](https://github.com/peco/peco/releases), find the version you want, and download the zip file. Unpack the zip file, and put the binary to somewhere you want (on UNIX-y systems, /usr/local/bin or the like). Make sure it has execution bits turned on. Yes, it is a single binary! You can put it anywhere you want :)

_THIS IS THE RECOMMENDED WAY_ (except for OS X homebrew users)

### Mac OS X / Homebrew

If you're on OS X and want to use homebrew:

```
brew install peco
```

### Windows (Chocolatey NuGet Users)

There's a third-party [peco package available](https://chocolatey.org/packages/peco) for Chocolatey NuGet.

```
C:\> choco install peco
```

### Building peco yourself

From the root of your cloned peco repository, run:

```
glide install
```

This installs the correct versions of peco's dependencies. Then build it:

```
go build cmd/peco/peco.go
```

This compiles a peco binary in the root of the cloned peco repository. Copy this file to an appropriate location.

### go get IS NOT RECOMMENDED

Please DO NOT use `go get` to install this tool. It bypasses the developers' intention of controlling the dependency versioning. 

# Command Line Options

### -h, --help

Display a help message

### --version

Display the version of peco

### --query <query>

Specifies the default query to be used upon startup. This is useful for scripts and functions where you can figure out before hand what the most likely query string is.

### --rcfile <filename>

Pass peco a configuration file, which currently must be a JSON file. If unspecified it will try a series of files by default. See `Configuration File` for the actual locations searched.

### -b, --buffer-size <num>

Limits the buffer size to `num`. This is an important feature when you are using peco against a possibly infinite stream, as it limits the number of lines that peco holds at any given time, preventing it from exhausting all the memory. By default the buffer size is unlimited.

### --null

WARNING: EXPERIMENTAL. This feature will probably stay, but the option name may change in the future.

Changes how peco interprets incoming data. When this flag is set, you may insert NUL ('\0') characters in your input. Anything before the NUL character is treated as the string to be displayed by peco and is used for matching against user query. Anything after the NUL character is used as the "result": i.e., when peco is about to exit, it displays this string instead of the original string displayed.

[Here's a simple example of how to use this feature](https://gist.github.com/mattn/3c7a14c1677ecb193acd)

### --initial-index

Specifies the initial line position upon start up. E.g. If you want to start out with the second line selected, set it to "1" (because the index is 0 based)

### --initial-filter `IgnoreCase|CaseSensitive|SmartCase|Regexp|Fuzzy`

Specifies the initial filter to use upon start up. You should specify the name of the filter like `IgnoreCase`, `CaseSensitive`, `SmartCase`, `Regexp` and `Fuzzy`. Default is `IgnoreCase`.

### --prompt

Specifies the query line's prompt string. When specified, takes precedence over the configuration file's `Prompt` section. The default value is `QUERY>`

### --layout `top-down|bottom-up`

Specifies the display layout. Default is `top-down`, where query prompt is at the top, followed by the list, then the system status message line. `bottom-up` changes this to the list first (displayed in reverse order), the query prompt, and then the system status message line.

For `percol` users, `--layout=bottom-up` is almost equivalent of `--prompt-bottom --result-bottom-up`.

### --select-1

When specified *and* the input contains exactly 1 line, peco skips prompting you for a choice, and selects the only line in the input and immediately exits.

If there are multiple lines in the input, the usual selection view is displayed.

### --on-cancel `success|error`

Specifies the exit status to use when the user cancels the query execution.
For historical and back-compatibility reasons, the default is `success`, meaning if the user cancels the query, the exit status is 0. When you choose `error`, peco will exit with a non-zero value.

### --selection-prefix `string`

When specified, peco uses the specified prefix instead of changing line color to indicate currently selected line(s). default is to use colors. This option is experimental

### --exec `string`

When specified, peco executes the specified external command (via shell), with peco's currently selected line(s) as its input from STDIN.

Upon exiting from the external command, the control goes back to peco where you can keep browsing your search buffer, and to possibly execute your external command repeatedly afterwards.

To exit out of peco when running in this mode, you must execute the Cancel command, usually the escape key.

# Configuration File

peco by default consults a few locations for the config files.

1. Location specified in --rcfile. If this doesn't exist, peco complains and exits
2. $XDG\_CONFIG\_HOME/peco/config.json
3. $HOME/.config/peco/config.json
4. for each directories listed in $XDG\_CONFIG\_DIRS, $DIR/peco/config.json
5. If all else fails, $HOME/.peco/config.json

Below are configuration sections that you may specify in your config file:

* [Global](#global)
* [Keymaps](#keymaps)
* [Styles](#styles)
* [CustomFilter](#customfilter)
* [CustomMatcher](#custommatcher)
* [Prompt](#prompt)
* [InitialMatcher](#initialmatcher)

## Global

Global configurations that change the global behavior.

### Prompt

You can change the query line's prompt, which is `QUERY>` by default.

```json
{
    "Prompt": "[peco]"
}
```

### InitialMatcher

*InitialMatcher* has been deprecated. Please use `InitialFilter` instead.

### InitialFilter

Specifies the filter name to start peco with. You should specify the name of the filter, such as `IgnoreCase`, `CaseSensitive`, `SmartCase`, `Regexp` and `Fuzzy`.

### StickySelection

```json
{
    "StickySelection": true
}
```

StickySelection allows selections to persist even between changes to the query.
For example, when you set this to true you can select a few lines, type in a 
new query, select those lines, and then delete the query. The result is all
the lines that you selected before and after the modification to the query are
left in tact.

Default value for StickySelection is false.

### OnCancel

```json
{
    "OnCancel": "error"
}
```

OnCancel is equivalent to `--on-cancel` command line option.

### MaxScanBufferSize

```json
{
    "MaxScanBufferSize": 256
}
```

Controls the buffer sized (in kilobytes) used by `bufio.Scanner`, which is
responsible for reading the input lines. If you believe that your input has
very long lines that prohibit peco from reading them, try increasing this number.

The same time, the default MaxScanBuferSize is 256kb.

## Keymaps

Example:

```json
{
    "Keymap": {
        "M-v": "peco.ScrollPageUp",
        "C-v": "peco.ScrollPageDown",
        "C-x,C-c": "peco.Cancel"
    }
}
```

### Key sequences

As of v0.2.0, you can use a list of keys (separated by comma) to register an action that is associated with a key sequence (instead of a single key). Please note that if there is a conflict in the key map, *the longest sequence always wins*. So In the above example, if you add another sequence, say, `C-x,C-c,C-c`, then the above `peco.Cancel` will never be invoked.

### Combined actions

As of v0.2.1, you can create custom combined actions. For example, if you find yourself repeatedly needing to select 4 lines out of the list, you may want to define your own action like this:

```json
{
    "Action": {
        "foo.SelectFour": [
            "peco.ToggleRangeMode",
            "peco.SelectDown",
            "peco.SelectDown",
            "peco.SelectDown",
            "peco.ToggleRangeMode"
        ]
    },
    "Keymap": {
        "M-f": "foo.SelectFour"
    }
}
```

This creates a new combined action `foo.SelectFour` (the format of the name is totally arbitrary, I just like to put namespaces), and assigns that action to `M-f`. When it's fired, it toggles the range selection mode and highlights 4 lines, and then goes back to waiting for your input.

As a similar example, a common idiom in emacs is that `C-c C-c` means "take the contents of this buffer and accept it", whatever that means.  This adds exactly that keybinding:

```json
{
    "Action": {
        "selectAllAndFinish": [
            "peco.SelectAll",
            "peco.Finish"
        ]
    },
    "Keymap": {
        "C-c,C-c": "selectAllAndFinish"
    }
}
```

### Available keys

Since v0.1.8, in addition to values below, you may put a `M-` prefix on any 
key item to use Alt/Option key as a mask.

| Name        | Notes |
|-------------|-------|
| C-a ... C-z | Control + whatever character |
| C-2 ... C-8 | Control + 2..8 |
| C-[         ||
| C-]         ||
| C-~         ||
| C-\_        ||
| C-\\\\      | Note that you need to escape the backslash |
| C-/         ||
| C-Space     ||
| F1 ... F12  ||
| Esc         ||
| Tab         ||
| Enter       ||
| Insert      ||
| Delete      ||
| BS          ||
| BS2         ||
| Home        ||
| End         ||
| Pgup        ||
| Pgdn        ||
| ArrowUp     ||
| ArrowDown   ||
| ArrowLeft   ||
| ArrowRight  ||
| MouseLeft   ||
| MouseMiddle ||
| MouseRight  ||


### Key workarounds

Some keys just... don't map correctly / too easily for various reasons. Here, we'll list possible workarounds for key sequences that are often asked for:


| You want this | Use this instead | Notes            |
|---------------|------------------|------------------|
| Shift+Tab     | M-\[,Z           | Verified on OS X |

### Available actions

| Name | Notes |
|------|-------|
| peco.ForwardChar        | Move caret forward 1 character |
| peco.BackwardChar       | Move caret backward 1 character |
| peco.ForwardWord        | Move caret forward 1 word |
| peco.BackwardWord       | Move caret backward 1 word|
| peco.BackToInitialFilter| Switch to first filter in the list |
| peco.BeginningOfLine    | Move caret to the beginning of line |
| peco.EndOfLine          | Move caret to the end of line |
| peco.EndOfFile          | Delete one character forward, otherwise exit from peco with failure status |
| peco.DeleteForwardChar  | Delete one character forward |
| peco.DeleteBackwardChar | Delete one character backward |
| peco.DeleteForwardWord  | Delete one word forward |
| peco.DeleteBackwardWord | Delete one word backward |
| peco.InvertSelection    | Inverts the selected lines |
| peco.KillBeginningOfLine | Delete the characters under the cursor backward until the beginning of the line |
| peco.KillEndOfLine      | Delete the characters under the cursor until the end of the line |
| peco.DeleteAll          | Delete all entered characters |
| peco.RefreshScreen      | Redraws the screen. Note that this effectively re-runs your query |
| peco.SelectPreviousPage | (DEPRECATED) Alias to ScrollPageUp |
| peco.SelectNextPage     | (DEPRECATED) Alias to ScrollPageDown |
| peco.ScrollPageDown     | Moves the selected line cursor for an entire page, downwards |
| peco.ScrollPageUp       | Moves the selected line cursor for an entire page, upwards |
| peco.SelectUp           | Moves the selected line cursor to one line above |
| peco.SelectDown         | Moves the selected line cursor to one line below |
| peco.SelectPrevious     | (DEPRECATED) Alias to SelectUp |
| peco.SelectNext         | (DEPRECATED) Alias to SelectDown |
| peco.ScrollLeft         | Scrolls the screen to the left |
| peco.ScrollRight        | Scrolls the screen to the right |
| peco.ToggleSelection    | Selects the current line, and saves it |
| peco.ToggleSelectionAndSelectNext | Selects the current line, saves it, and proceeds to the next line |
| peco.ToggleSingleKeyJump | Enables SingleKeyJump mode a.k.a. "hit-a-hint" |
| peco.SelectNone         | Remove all saved selections |
| peco.SelectAll          | Selects the all line, and save it  |
| peco.SelectVisible      | Selects the all visible line, and save it |
| peco.ToggleSelectMode   | (DEPRECATED) Alias to ToggleRangeMode |
| peco.CancelSelectMode   | (DEPRECATED) Alias to CancelRangeMode |
| peco.ToggleQuery        | Toggle list between filterd by query and not filterd. |
| peco.ToggleRangeMode   | Start selecting by range, or append selecting range to selections |
| peco.CancelRangeMode   | Finish selecting by range and cancel range selection |
| peco.RotateMatcher     | (DEPRECATED) Use peco.RotateFilter |
| peco.RotateFilter       | Rotate between filters (by default, ignore-case/no-ignore-case)|
| peco.Finish             | Exits from peco with success status |
| peco.Cancel             | Exits from peco with failure status, or cancel select mode |


### Default Keymap

Note: If in case below keymap seems wrong, check the source code in [keymap.go](https://github.com/peco/peco/blob/master/keymap.go) (look for NewKeymap).

|Key|Action|
|---|------|
|Esc|peco.Cancel|
|C-c|peco.Cancel|
|Enter|peco.Finish|
|C-f|peco.ForwardChar|
|C-a|peco.BeginningOfLine|
|C-b|peco.BackwardChar|
|C-d|peco.DeleteForwardChar|
|C-e|peco.EndOfLine|
|C-k|peco.KillEndOfLine|
|C-u|peco.KillBeginningOfLine|
|BS|peco.DeleteBackwardChar|
|C-8|peco.DeleteBackwardChar|
|C-w|peco.DeleteBackwardWord|
|C-g|peco.SelectNone|
|C-n|peco.SelectDown|
|C-p|peco.SelectUp|
|C-r|peco.RotateFilter|
|C-t|peco.ToggleQuery|
|C-Space|peco.ToggleSelectionAndSelectNext|
|ArrowUp|peco.SelectUp|
|ArrowDown|peco.SelectDown|
|ArrowLeft|peco.ScrollPageUp|
|ArrowRight|peco.ScrollPageDown|

## Styles

For now, styles of following 5 items can be customized in `config.json`.

```json
{
    "Style": {
        "Basic": ["on_default", "default"],
        "SavedSelection": ["bold", "on_yellow", "white"],
        "Selected": ["underline", "on_cyan", "black"],
        "Query": ["yellow", "bold"],
        "Matched": ["red", "on_blue"]
    }
}
```

- `Basic` for not selected lines
- `SavedSelection` for lines of saved selection
- `Selected` for a currently selecting line
- `Query` for a query line
- `Matched` for a query matched word

### Foreground Colors

- `"black"` for `termbox.ColorBlack`
- `"red"` for `termbox.ColorRed`
- `"green"` for `termbox.ColorGreen`
- `"yellow"` for `termbox.ColorYellow`
- `"blue"` for `termbox.ColorBlue`
- `"magenta"` for `termbox.ColorMagenta`
- `"cyan"` for `termbox.ColorCyan`
- `"white"` for `termbox.ColorWhite`

### Background Colors

- `"on_black"` for `termbox.ColorBlack`
- `"on_red"` for `termbox.ColorRed`
- `"on_green"` for `termbox.ColorGreen`
- `"on_yellow"` for `termbox.ColorYellow`
- `"on_blue"` for `termbox.ColorBlue`
- `"on_magenta"` for `termbox.ColorMagenta`
- `"on_cyan"` for `termbox.ColorCyan`
- `"on_white"` for `termbox.ColorWhite`

### Attributes

- `"bold"` for fg: `termbox.AttrBold`
- `"underline"` for fg: `termbox.AttrUnderline`
- `"reverse"` for fg: `termbox.AttrReverse`
- `"on_bold"` for bg: `termbox.AttrBold` (this attribute actually makes the background blink on some platforms/environments, e.g. linux console, xterm...)

## CustomFilter

This is an experimental feature. Please note that some details of this specification may change

By default `peco` comes with `IgnoreCase`, `CaseSensitive`, `SmartCase`, `Regexp` and `Fuzzy` filters, but since v0.1.3, it is possible to create your own custom filter.

The filter will be executed via  `Command.Run()` as an external process, and it will be passed the query values in the command line, and the original unaltered buffer is passed via `os.Stdin`. Your filter must perform the matching, and print out to `os.Stdout` matched lines. You filter MAY be called multiple times if the buffer
given to peco is big enough. See `BufferThreshold` below.

Note that currently there is no way for the custom filter to specify where in the line the match occurred, so matched portions in the string WILL NOT BE HIGHLIGHTED.

The filter does not need to be a go program. It can be a perl/ruby/python/bash script, or anything else that is executable.

Once you have a filter, you must specify how the matcher is spawned:

```json
{
    "CustomFilter": {
        "MyFilter": {
            "Cmd": "/path/to/my-matcher",
            "Args": [ "$QUERY" ],
            "BufferThreshold": 100
        }
    }
}
```

`Cmd` specifies the command name. This must be searcheable via `exec.LookPath`.

Elements in the `Args` section are string keys to array of program arguments. The special token `$QUERY` will be replaced with the unaltered query as the user typed in (i.e. multiple-word queries will be passed as a single string). You may pass in any other arguments in this array. If you omit this in your config, a default value of `[]string{"$QUERY"}` will be used

`BufferThreshold` specifies that the filter command should be invoked when peco has this many lines to process
in the buffer. For example, if you are using peco against a 1000-line input, and your `BufferThreshold` is 100 (which is the default), then your filter will be invoked 10 times. For obvious reasons, the larger this threshold is, the faster the overall performance will be, but the longer you will have to wait to see the filter results.

You may specify as many filters as you like in the `CustomFilter` section.

### Examples

* [An example of a simple perl regexp matcher](https://gist.github.com/mattn/24712964da6e3112251c)
* [An example using migemogrep Japanese grep using latin-1 chars](https://github.com/peco/peco/wiki/CustomMatcher)

## Layout

See --layout.

## SingleKeyJump

```
{
  "SingleKeyJump": {
    "ShowPrefix": true
  }
}
```

## SelectionPrefix

`SelectionPrefix` is equivalent to using `--selection-prefix` in the command line.

```
{
  "SelectionPrefix": ">"
}
```

# FAQ

## Does peco work on (msys2|cygwin)?

No. https://github.com/peco/peco/issues/336#issuecomment-243939696
(Updated Feb 23, 2017: "Maybe" on cygwin https://github.com/peco/peco/issues/336#issuecomment-281912949)

## Non-latin fonts (e.g. Japanese) look weird on my Windows machine...?

Are you using raster fonts? https://github.com/peco/peco/issues/341

## Seeing escape sequences `[200~` and `[201~` when pasting text?

Disable bracketed paste mode. https://github.com/peco/peco/issues/417

# Hacking

First, fork this repo, and get your clone locally.

1. Make sure you have [go](http://golang.org) installed, with GOPATH appropriately set
2. Make sure you have `make` installed
3. Run `make installdeps` (You only need to do this once, or when you update glide.yml/glide.lock)

To test, run

```
make test
```

To build, run

```
make build
```

This will create a `peco` binary in `$(RELEASE_DIR)/peco_$(GOOS)_$(GOARCH)/peco$(SUFFIX)`. Or, of course, you can just run

```
go build cmd/peco/peco.go
```

which will create the binary in the local directory.

# TODO

Test it. In doing so, we may change the repo structure

Implement all(?) of the original percol options

# AUTHORS

* Daisuke Maki (lestrrat)
* mattn
* syohex

# CONTRIBUTORS

* HIROSE Masaaki
* Joel Segerlind
* Lukas Lueg
* Mitsuoka Mimura
* Ryota Arai
* Shinya Ohyanagi
* Takashi Kokubun
* Yuya Takeyama
* cho45
* cubicdaiya
* kei\_q
* negipo
* sona\_tar
* sugyan
* swdyh
* MURAOKA Taro (kaoriya/koron), for aho-corasick search
* taichi, for the gif working on Windows
* uobikiemukot
* Samuel Lemaitre
* Yousuke Ushiki
* Linda\_pp
* Tomohiro Nishimura (Sixeight)

# Notes

Obviously, kudos to the original percol: https://github.com/mooz/percol
Much code stolen from https://github.com/mattn/gof

# Table of Contents

* [peco](#peco)
* [Description](#description)
	* [Demo](#demo)
* [Features](#features)
	* [Incremental Search](#incremental-search)
	* [Select Multiple Lines](#select-multiple-lines)
	* [Select Range Of Lines](#select-range-of-lines)
	* [Select Filters](#select-filters)
	* [Selectable Layout](#selectable-layout)
	* [Works on Windows!](#works-on-windows)
* [Installation](#installation)
	* [Just want the binary?](#just-want-the-binary)
	* [Mac OS X / Homebrew](#mac-os-x--homebrew)
	* [Windows (Chocolatey NuGet Users)](#windows-chocolatey-nuget-users)
	* [Building peco yourself](#building-peco-yourself)
	* [go get IS NOT RECOMMENDED](#go-get-is-not-recommended)
* [Command Line Options](#command-line-options)
	* [-h, --help](#-h---help)
	* [--version](#--version)
	* [--query <query>](#--query-query)
	* [--rcfile <filename>](#--rcfile-filename)
	* [-b, --buffer-size <num>](#-b---buffer-size-num)
	* [--null](#--null)
	* [--initial-index](#--initial-index)
	* [--initial-filter `IgnoreCase|CaseSensitive|SmartCase|Regexp|Fuzzy`](#--initial-filter-ignorecasecasesensitivesmartcaseregexpfuzzy)
	* [--prompt](#--prompt)
	* [--layout `top-down|bottom-up`](#--layout-top-downbottom-up)
	* [--select-1](#--select-1)
	* [--on-cancel `success|error`](#--on-cancel-successerror)
        * [--selection-prefix `string`](#--selection-prefix-string)
        * [--exec `string`](#--exec-string)
* [Configuration File](#configuration-file)
	* [Global](#global)
		* [Prompt](#prompt)
		* [InitialMatcher](#initialmatcher)
		* [InitialFilter](#initialfilter)
		* [StickySelection](#stickyselection)
		* [OnCancel](#oncancel)
	* [Keymaps](#keymaps)
		* [Key sequences](#key-sequences)
		* [Combined actions](#combined-actions)
		* [Available keys](#available-keys)
		* [Key workarounds](#key-workarounds)
		* [Available actions](#available-actions)
		* [Default Keymap](#default-keymap)
	* [Styles](#styles)
		* [Foreground Colors](#foreground-colors)
		* [Background Colors](#background-colors)
		* [Attributes](#attributes)
	* [CustomFilter](#customfilter)
		* [Examples](#examples)
	* [Layout](#layout)
	* [SingleKeyJump](#singlekeyjump)
	* [SelectionPrefix](#selectionprefix)
* [FAQ](#faq)
	* [Does peco work on (msys2|cygwin)?](#does-peco-work-on-msys2cygwin)
	* [Non-latin fonts (e.g. Japanese) look weird on my Windows machine...?](#non-latin-fonts-eg-japanese-look-weird-on-my-windows-machine)
* [Hacking](#hacking)
* [TODO](#todo)
* [AUTHORS](#authors)
* [CONTRIBUTORS](#contributors)
* [Notes](#notes)
* [Table of Contents](#table-of-contents)
