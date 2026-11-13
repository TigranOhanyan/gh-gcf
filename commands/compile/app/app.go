package app

import (
	"os"

	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/logging"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/executor"
)

func NewCompileApp() (cmd *cobra.Command) {
	var configBranch string
	var stateBranch string
	var tickID string

	cmd = &cobra.Command{
		Use:   "app",
		Short: "Record configured apps that are installed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			ctx := logging.With(cmd.Context(), zap.String("tick_id", tickID))
			cmd.SetContext(ctx)
			dir, err := os.Getwd()
			if err != nil {
				logging.From(cmd.Context()).Error("current directory", zap.Error(err))
				return
			}
			err = commands.RequireCheckout(cmd.Context(), dir)
			if err != nil {
				logging.From(cmd.Context()).Error("factory repo", zap.Any("path", dir), zap.Error(err))
				return
			}
			exec := executor.StepExecutor{
				FactoryDir:   dir,
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
	flags.StringVar(&configBranch, "config-branch", "main", "branch that holds config")
	flags.StringVar(&stateBranch, "state-branch", "state", "branch that holds state")
	flags.StringVar(&tickID, "tick", "", "tick id this step records")
	err := cmd.MarkFlagRequired("tick")
	if err != nil {
		logging.From(cmd.Context()).Error("tick flag", zap.Error(err))
	}
	return
}
