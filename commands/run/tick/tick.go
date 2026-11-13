package tick

import (
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/logging"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

func NewRunTick() (cmd *cobra.Command) {
	var configBranch string
	var stateBranch string

	cmd = &cobra.Command{
		Use:   "tick",
		Short: "Record a tick and print its id",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
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
			tick := tickExecutor{
				FactoryDir:   dir,
				ConfigBranch: configBranch,
				StateBranch:  stateBranch,
			}
			id, err := tick.CreateTick(cmd.Context())
			if err != nil {
				return
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), id)
			return
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&configBranch, "config-branch", "main", "branch that holds config")
	flags.StringVar(&stateBranch, "state-branch", "state", "branch that holds state")
	return
}
