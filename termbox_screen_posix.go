// +build !tcell,!windows

package peco

import "github.com/nsf/termbox-go"

func (t *Termbox) PostInit(cfg *Config) error {
	// This has no effect on Windows,
	// because termbox.SetOutputMode always sets termbox.OutputNormal on Windows.
	if cfg.Use256Color {
		termbox.SetOutputMode(termbox.Output256)
	}

	return nil
}
