//go:build !windows
// +build !windows

package peco

// PostInit is a no-op on POSIX systems. tcell auto-detects
// color capability via terminfo.
func (t *TcellScreen) PostInit(cfg *Config) error {
	return nil
}
