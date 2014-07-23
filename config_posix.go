// +build !darwin,!windows

package peco

import (
	"fmt"
	"os"
)

func homedir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("error: Environment variable HOME not set")
	}

	return home, nil
}
