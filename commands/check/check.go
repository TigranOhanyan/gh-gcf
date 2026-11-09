package check

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/check/builder"
	"github.com/TigranOhanyan/gh-gcf/commands/check/fixer"
	"github.com/TigranOhanyan/gh-gcf/commands/check/specification"
	"github.com/TigranOhanyan/gh-gcf/commands/check/standards"
)

func NewCheck() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "check",
		Short: "Record a finished builder",
	}
	builderCmd := builder.NewCheckBuilder()
	cmd.AddCommand(builderCmd)
	standardsCmd := standards.NewCheckStandards()
	cmd.AddCommand(standardsCmd)
	specificationCmd := specification.NewCheckSpecification()
	cmd.AddCommand(specificationCmd)
	fixerCmd := fixer.NewCheckFixer()
	cmd.AddCommand(fixerCmd)
	return
}
