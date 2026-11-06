package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"charm.land/log/v2"
	"mvdan.cc/sh/v3/shell"
)

// Git runs one shell line of git arguments in dir and returns stdout.
// A non-zero exit includes stderr in the error.
// A quoted value stays one argument. Variables in line are left unset.
func Git(ctx context.Context, dir string, line string) (output []byte, err error) {
	args, err := shell.Fields(line, unset)
	if err != nil {
		log.Error("split git", "line", line, "err", err)
		return
	}
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

func quoteWord(word string) (quoted string) {
	replaced := strings.ReplaceAll(word, "'", `'"'"'`)
	quoted = "'" + replaced + "'"
	return
}

func findCheckout(output []byte, branch string) (path string, found bool) {
	ref := "refs/heads/" + branch
	text := string(output)
	records := strings.Split(text, "\n\n")
	for _, record := range records {
		path, found = checkoutRecord(record, ref)
		if found {
			return
		}
	}
	return
}

func checkoutRecord(record string, ref string) (path string, found bool) {
	var branchRef string
	lines := strings.Split(record, "\n")
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		key, value, hasValue := strings.Cut(line, " ")
		if !hasValue {
			continue
		}
		isWorktree := key == "worktree"
		if isWorktree {
			path = value
		}
		isBranch := key == "branch"
		if isBranch {
			branchRef = value
		}
	}
	found = branchRef == ref
	if !found {
		path = ""
	}
	return
}
