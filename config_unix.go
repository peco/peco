// +build darwin freebsd linux netbsd openbsd
package peco

import (
	"fmt"
	"os"
)

func homedir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("Environment variable HOME not set")
	}

	return home, nil
}