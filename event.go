package peco

import "github.com/peco/peco/internal/keyseq"

// EventType classifies the type of terminal event.
type EventType uint8

const (
	// EventKey is a keyboard event
	EventKey EventType = iota
	// EventResize is a terminal resize event
	EventResize
	// EventError is an error event
	EventError
)

// Event is peco's internal event type, replacing termbox.Event.
// This decouples all code outside the screen adapter from the
// terminal library.
type Event struct {
	Type EventType
	Key  keyseq.KeyType
	Ch   rune
	Mod  keyseq.ModifierKey
}
