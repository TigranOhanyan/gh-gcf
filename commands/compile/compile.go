package compile

import (
	"os"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/executor"
)

func NewCompile() (cmd *cobra.Command) {
	var configBranch string
	var stateBranch string
	var tickID string

	cmd = &cobra.Command{
		Use:   "compile",
		Short: "Record configured members in state",
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
			exec := executor.StepExecutor{
				FactoryDir:   dir,
				ConfigBranch: configBranch,
				StateBranch:  stateBranch,
				TickID:       tickID,
			}
			step := compileStep{}
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
