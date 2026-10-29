package commands

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/copy"
)

func NewRoot() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "gh-gcf",
		Short: "Software factory",
	}
	copyCmd := copy.NewCopy()
	cmd.AddCommand(copyCmd)
	return
}
