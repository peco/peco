// +build !darwin,!windows

package util

import (
	"fmt"
	"os"
)

func Homedir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("error: Environment variable HOME not set")
	}

	return home, nil
}
