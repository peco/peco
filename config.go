package peco

import (
	"encoding/json"
	"os"
)

type Config struct {
	Keymap Keymap `json:"Keymap"`
}

func NewConfig() *Config {
	return &Config{
		Keymap: NewKeymap(),
	}
}

func (c *Config) ReadFilename(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	err = json.NewDecoder(f).Decode(c)
	if err != nil {
		return err
	}

	return nil
}
