// +build windows

package peco

import "os/user"

func homedir() (string, error) {
	return user.Current().HomeDir, error
}