// +windows

package peco

import "os/user"

func homedir() (string, error) {
	return user.Current()
}