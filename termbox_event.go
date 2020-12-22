package peco

import (
	"unicode/utf8"

	"github.com/nsf/termbox-go"
)

type Modifier = termbox.Modifier
const (
	ModAlt Modifier = termbox.ModAlt
	ModMotion Modifier = termbox.ModMotion
)

type termboxEvent struct {
	raw termbox.Event
}

func (e *termboxEvent) IsError() bool {
	return e.raw.Type == termbox.EventError
}

func (e *termboxEvent) IsKey() bool {
	return e.raw.Type == termbox.EventKey
}

func (e *termboxEvent) IsResize() bool {
	return e.raw.Type == termbox.EventResize
}

func (e *termboxEvent) IsMouse() bool {
	return e.raw.Type == termbox.EventMouse
}

func (e *termboxEvent) IsInterrupt() bool {
	return e.raw.Type == termbox.EventInterrupt
}

func (e *termboxEvent) Rune() rune {
	if !e.IsKey() {
		return utf8.RuneError
	}
	return e.raw.Ch
}

func (e *termboxEvent) SetRune(r rune) {
	e.raw.Ch = r
}

func (e *termboxEvent) KeyCode() KeyCode {
	if !e.IsKey() {
		return 0x00 // XXX no "invalid" key code
	}

	return e.raw.Key
}

func (e *termboxEvent) Modifier() Modifier {
	return e.raw.Mod
}

func (e *termboxEvent) HasModifier(m Modifier) bool {
	return (e.raw.Mod & m == 1)
}

func (e *termboxEvent) SetModifier(m Modifier, on bool) {
	if on {
		e.raw.Mod |= m
	} else {
		e.raw.Mod &^= m
	}
}

func RuneEvent(r rune) Event {
	return &termboxEvent{
		termbox.Event{
			Ch: r,
		},
	}
}

func KeyEvent(k KeyCode) Event {
	return &termboxEvent{
		termbox.Event{
			Type: termbox.EventKey,
			Key:  k,
		},
	}
}
