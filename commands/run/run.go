package run

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/run/builder"
	"github.com/TigranOhanyan/gh-gcf/commands/run/fixer"
	"github.com/TigranOhanyan/gh-gcf/commands/run/specification"
	"github.com/TigranOhanyan/gh-gcf/commands/run/standards"
	"github.com/TigranOhanyan/gh-gcf/commands/run/tick"
)

func NewRun() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "run",
		Short: "Start an agent for a development",
	}
	tickCmd := tick.NewRunTick()
	cmd.AddCommand(tickCmd)
	builderCmd := builder.NewRunBuilder()
	cmd.AddCommand(builderCmd)
	standardsCmd := standards.NewRunStandards()
	cmd.AddCommand(standardsCmd)
	specificationCmd := specification.NewRunSpecification()
	cmd.AddCommand(specificationCmd)
	fixerCmd := fixer.NewRunFixer()
	cmd.AddCommand(fixerCmd)
	return
}
