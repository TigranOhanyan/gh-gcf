package setup

import (
	"os"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/apps"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/config"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/development"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/label"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/repo"
)

func NewSetup() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "setup",
		Short: "Record config for app repos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			dir, err := os.Getwd()
			if err != nil {
				log.Error("current directory", "err", err)
				return
			}
			err = config.Setup(cmd.Context(), dir)
			return
		},
	}
	developmentCmd := development.NewSetupDevelopment()
	cmd.AddCommand(developmentCmd)
	repoCmd := repo.NewSetupRepo()
	cmd.AddCommand(repoCmd)
	labelCmd := label.NewSetupLabel()
	cmd.AddCommand(labelCmd)
	appsCmd := apps.NewSetupApps()
	cmd.AddCommand(appsCmd)
	return
}
