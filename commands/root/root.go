package root

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/app"
	"github.com/TigranOhanyan/gh-gcf/commands/bind"
	"github.com/TigranOhanyan/gh-gcf/commands/compile"
	compiledevelopment "github.com/TigranOhanyan/gh-gcf/commands/compile/development"
	"github.com/TigranOhanyan/gh-gcf/commands/label"
	"github.com/TigranOhanyan/gh-gcf/commands/poll"
	"github.com/TigranOhanyan/gh-gcf/commands/run"
	rundevelopment "github.com/TigranOhanyan/gh-gcf/commands/run/development"
	"github.com/TigranOhanyan/gh-gcf/commands/setup"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/development"
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
	compileDevelopmentCmd := compiledevelopment.NewCompileDevelopment()
	compileCmd.AddCommand(compileDevelopmentCmd)
	cmd.AddCommand(compileCmd)
	labelCmd := label.NewLabel()
	cmd.AddCommand(labelCmd)
	appCmd := app.NewApp()
	cmd.AddCommand(appCmd)
	setupCmd := setup.NewSetup()
	developmentCmd := development.NewSetupDevelopment()
	setupCmd.AddCommand(developmentCmd)
	cmd.AddCommand(setupCmd)
	runCmd := run.NewRun()
	runDevelopmentCmd := rundevelopment.NewRunDevelopment()
	runCmd.AddCommand(runDevelopmentCmd)
	cmd.AddCommand(runCmd)
	pollCmd := poll.NewPoll()
	cmd.AddCommand(pollCmd)
	return
}
