//go:build windows

package util

import (
	"context"
	"os/exec"
)

func Shell(ctx context.Context, cmd ...string) *exec.Cmd {
	const shellpath = `cmd`
	const shellopt = `/c`

	args := make([]string, len(cmd)+1)
	args[0] = shellopt
	for i := range cmd {
		args[i+1] = cmd[i]
	}

	return exec.CommandContext(ctx, shellpath, args...)
}
