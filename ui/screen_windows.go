package ui

import "github.com/nsf/termbox-go"

func (t *Termbox) PostInit() error {
	// Windows handle Esc/Alt self
	termbox.SetInputMode(termbox.InputEsc | termbox.InputAlt)

	return nil
}
