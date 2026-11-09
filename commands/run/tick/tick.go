package tick

import (
	"fmt"
	"os"

	"charm.land/log/v2"
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
				log.Error("current directory", "err", err)
				return
			}
			err = commands.RequireCheckout(cmd.Context(), dir)
			if err != nil {
				log.Error("factory repo", "path", dir, "err", err)
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
