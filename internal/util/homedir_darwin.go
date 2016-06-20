package util

import (
	"errors"
	"os"
)

func Homedir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("environment variable HOME not set")
	}

	return home, nil
}
