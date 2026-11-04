package app

import (
	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/executor"
)

func NewApp() (cmd *cobra.Command) {
	var factoryDir string
	var configBranch string
	var stateBranch string
	var tickID string

	cmd = &cobra.Command{
		Use:   "app",
		Short: "Record configured apps that are installed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			exec := executor.StepExecutor{
				FactoryDir:   factoryDir,
				ConfigBranch: configBranch,
				StateBranch:  stateBranch,
				TickID:       tickID,
			}
			step := appStep{}
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
