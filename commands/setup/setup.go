package setup

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/apps"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/development"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/label"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/repo"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/tui"
)

func NewSetup() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "setup",
		Short: "Record config for app repos",
	}
	developmentCmd := development.NewSetupDevelopment()
	cmd.AddCommand(developmentCmd)
	repoCmd := repo.NewSetupRepo()
	cmd.AddCommand(repoCmd)
	labelCmd := label.NewSetupLabel()
	cmd.AddCommand(labelCmd)
	appsCmd := apps.NewSetupApps()
	cmd.AddCommand(appsCmd)
	tuiCmd := tui.NewTui()
	cmd.AddCommand(tuiCmd)
	return
}
