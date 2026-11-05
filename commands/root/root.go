package root

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/app"
	"github.com/TigranOhanyan/gh-gcf/commands/bind"
	"github.com/TigranOhanyan/gh-gcf/commands/compile"
	"github.com/TigranOhanyan/gh-gcf/commands/label"
	"github.com/TigranOhanyan/gh-gcf/commands/setup"
	"github.com/TigranOhanyan/gh-gcf/commands/tick"
)

func NewRoot() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "gh-gcf",
		Short: "Software factory",
	}
	bindCmd := bind.NewBind()
	cmd.AddCommand(bindCmd)
	tickCmd := tick.NewTick()
	cmd.AddCommand(tickCmd)
	compileCmd := compile.NewCompile()
	cmd.AddCommand(compileCmd)
	labelCmd := label.NewLabel()
	cmd.AddCommand(labelCmd)
	appCmd := app.NewApp()
	cmd.AddCommand(appCmd)
	setupCmd := setup.NewSetup()
	cmd.AddCommand(setupCmd)
	return
}
