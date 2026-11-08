package check

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/check/builder"
	"github.com/TigranOhanyan/gh-gcf/commands/check/fixer"
)

func NewCheck() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "check",
		Short: "Record a finished builder",
	}
	builderCmd := builder.NewCheckBuilder()
	cmd.AddCommand(builderCmd)
	fixerCmd := fixer.NewCheckFixer()
	cmd.AddCommand(fixerCmd)
	return
}
