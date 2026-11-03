package label

import (
	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/executor"
)

func NewLabel() (cmd *cobra.Command) {
	var factoryDir string
	var configBranch string
	var stateBranch string
	var tickID string

	cmd = &cobra.Command{
		Use:   "label",
		Short: "Ensure configured labels exist",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			app, err := loadAppAuth()
			if err != nil {
				return
			}
			var client *github.Client
			needsUserToken := app.key == nil
			if needsUserToken {
				client, err = newGitHubClient(cmd.Context())
				if err != nil {
					return
				}
			}
			exec := executor.StepExecutor{
				FactoryDir:   factoryDir,
				ConfigBranch: configBranch,
				StateBranch:  stateBranch,
				TickID:       tickID,
			}
			step := labelStep{GitHub: client, app: app}
			err = exec.Execute(cmd.Context(), step)
			return
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&factoryDir, "factory-repo", "", "path to the factory checkout")
	flags.StringVar(&configBranch, "config-branch", "main", "branch that holds config")
	flags.StringVar(&stateBranch, "state-branch", "state", "branch that holds state")
	flags.StringVar(&tickID, "tick", "", "tick id this step records")
	err := cmd.MarkFlagRequired("tick")
	if err != nil {
		log.Error("tick flag", "err", err)
	}
	return
}
