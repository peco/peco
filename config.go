package peco

import (
	"encoding/json"
	"os"
)

type Config struct {
	Keymap  Keymap   `json:"Keymap"`
	Matcher string   `json:"Matcher"`
	Style   StyleSet `json:"Style"`
}

func NewConfig() *Config {
	return &Config{
		Keymap:  NewKeymap(),
		Matcher: IgnoreCaseMatch,
		Style:   NewStyleSet(),
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
