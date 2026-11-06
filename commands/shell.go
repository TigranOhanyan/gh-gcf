package commands

import (
	"context"

	"charm.land/log/v2"
	"github.com/spf13/cobra"
	"mvdan.cc/sh/v3/shell"
)

// Cmd is a cobra command that can be run from one shell line.
type Cmd struct {
	*cobra.Command
}

// NewCmd wraps cmd so Shell can run it from one shell line.
func NewCmd(cmd *cobra.Command) (wrapped Cmd) {
	wrapped.Command = cmd
	return
}

// Shell splits line into arguments and runs the command.
// A quoted value stays one argument. Variables in line are left unset.
func (c Cmd) Shell(ctx context.Context, line string) (err error) {
	args, err := shell.Fields(line, unset)
	if err != nil {
		log.Error("split command", "err", err)
		return
	}
	root := c.Root()
	root.SetArgs(args)
	err = root.ExecuteContext(ctx)
	return
}

func unset(string) string { return "" }
