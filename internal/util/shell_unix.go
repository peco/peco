//go:build !windows

package util //nolint:revive

import (
	"context"
	"os/exec"
)

func Shell(ctx context.Context, cmd ...string) *exec.Cmd {
	const shellpath = `/bin/sh`
	const shellopt = `-c`

	args := make([]string, len(cmd)+1)
	args[0] = shellopt
	for i := range cmd {
		args[i+1] = cmd[i]
	}

	return exec.CommandContext(ctx, shellpath, args...)
}
