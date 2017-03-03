// +build windows

package util

import "os/exec"

func Shell(cmd ...string) *exec.Cmd {
	const shellpath = `cmd`
	const shellopt  = `/c`

	args := make([]string, len(cmd) + 1)
	args[0] = shellopt
	for i := 0; i < len(cmd); i++ {
		args[i+1] = cmd[i]
	}
	
	return exec.Command(shellpath, args...)
}
