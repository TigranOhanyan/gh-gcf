package root

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/bind"
	"github.com/TigranOhanyan/gh-gcf/commands/check"
	"github.com/TigranOhanyan/gh-gcf/commands/compile"
	"github.com/TigranOhanyan/gh-gcf/commands/run"
	"github.com/TigranOhanyan/gh-gcf/commands/setup"
)

func NewRoot() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "gh-gcf",
		Short: "Software factory",
	}
	bindCmd := bind.NewBind()
	cmd.AddCommand(bindCmd)
	compileCmd := compile.NewCompile()
	cmd.AddCommand(compileCmd)
	setupCmd := setup.NewSetup()
	cmd.AddCommand(setupCmd)
	runCmd := run.NewRun()
	cmd.AddCommand(runCmd)
	checkCmd := check.NewCheck()
	cmd.AddCommand(checkCmd)
	return
}
