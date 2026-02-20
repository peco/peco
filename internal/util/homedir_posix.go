//go:build !darwin && !windows

package util

import (
	"errors"
	"os"
)

// Homedir returns the current user's home directory from the HOME environment variable.
func Homedir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("environment variable HOME not set")
	}

	return home, nil
}
