package compile

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/compile/app"
	"github.com/TigranOhanyan/gh-gcf/commands/compile/development"
	"github.com/TigranOhanyan/gh-gcf/commands/compile/label"
)

func NewCompile() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "compile",
		Short: "Copy a flow from config into state",
	}
	appCmd := app.NewCompileApp()
	cmd.AddCommand(appCmd)
	labelCmd := label.NewCompileLabel()
	cmd.AddCommand(labelCmd)
	developmentCmd := development.NewCompileDevelopment()
	cmd.AddCommand(developmentCmd)
	return
}
