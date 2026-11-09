package builder

import (
	"os"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/executor"
	"github.com/TigranOhanyan/gh-gcf/commands/run/agent"
)

func NewRunBuilder() (cmd *cobra.Command) {
	var configBranch string
	var stateBranch string
	var tickID string

	cmd = &cobra.Command{
		Use:   "builder",
		Short: "Start the builder for an open issue with a development's starting label",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			dir, err := os.Getwd()
			if err != nil {
				log.Error("current directory", "err", err)
				return
			}
			err = commands.RequireCheckout(cmd.Context(), dir)
			if err != nil {
				log.Error("factory repo", "path", dir, "err", err)
				return
			}
			app, err := commands.LoadFactoryApp()
			if err != nil {
				return
			}
			var client *github.Client
			needsUserToken := !app.Configured()
			if needsUserToken {
				client, err = commands.UserGitHubClient(cmd.Context())
				if err != nil {
					return
				}
			}
			cursor, err := agent.NewCursorClient()
			if err != nil {
				return
			}
			exec := executor.StepExecutor{
				FactoryDir:   dir,
				ConfigBranch: configBranch,
				StateBranch:  stateBranch,
				TickID:       tickID,
			}
			step := builderStep{GitHub: client, app: app, cursor: cursor}
			err = exec.Execute(cmd.Context(), step)
			return
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&configBranch, "config-branch", "main", "branch that holds config")
	flags.StringVar(&stateBranch, "state-branch", "state", "branch that holds state")
	flags.StringVar(&tickID, "tick", "", "tick id this step records")
	err := cmd.MarkFlagRequired("tick")
	if err != nil {
		log.Error("tick flag", "err", err)
	}
	return
}
