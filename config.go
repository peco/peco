package peco

import (
	"encoding/json"
	"io/ioutil"
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

	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	err = json.Unmarshal(buf, c)
	if err != nil {
		return err
	}

	return nil
}
