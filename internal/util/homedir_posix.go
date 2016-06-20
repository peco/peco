// +build !darwin,!windows

package util

import (
	"errors"
	"os"
)

func Homedir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("error: Environment variable HOME not set")
	}

	return home, nil
}
