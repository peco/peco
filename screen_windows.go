package peco

// PostInit is a no-op on Windows. tcell handles input mode automatically.
func (t *TcellScreen) PostInit(cfg *Config) error {
	return nil
}
