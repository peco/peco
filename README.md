peco
======

Simplistic interfacting filtering tool

Description
===========

peco is based on [percol](https://github.com/mooz/percol). The idea is that percol was darn useful, but I wanted a tool that was a single binary.

peco is written in Go, and as of this writing only implements the basic filtering feature (mainly because that's the only thing I use -- you're welcome to send me pull requests to make peco more compatible with percol). I have also been told that peco even works on windows :)

Installation
============

```
go get github.com/lestrrat/peco/cmd/peco/
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

TODO

Configuration File
==================

Currently only keymaps are supported:

```json
{
    "Keymap": {
        "C-p": "peco.SelectPrevious",
        "C-n": "peco.SelectNext",
    }
}
```

Filtering
=========

After you laungu peco, type somethig in. It will be matched against the
text you fed to peco, and the results will be filtered.

Navigation
==========

Use the left, right, up, and down arrow keys to navigate through all the results

TODO
====

Test it. In doing so, we may change the repo structure

Implement all(?) of the original percol options

Notes
=====

Much code stolen from https://github.com/mattn/gof
