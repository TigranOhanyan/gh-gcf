package bind

import (
	"context"
	"fmt"
	"os/exec"
)

var worktreesGitignore = []byte("*\n!.gitignore\n!internal_use_only\n")

func git(ctx context.Context, dir string, args ...string) (output []byte, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err = cmd.Output()
	if err == nil {
		return
	}
	exitErr, isExit := err.(*exec.ExitError)
	if isExit {
		detail := string(exitErr.Stderr)
		err = fmt.Errorf("%w: %s", err, detail)
	}
	return
}
