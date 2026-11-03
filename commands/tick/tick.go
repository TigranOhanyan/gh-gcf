package tick

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewTick() (cmd *cobra.Command) {
	var factoryDir string
	var configBranch string
	var stateBranch string

	cmd = &cobra.Command{
		Use:   "tick",
		Short: "Record a tick and print its id",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			tick := tickExecutor{
				FactoryDir:   factoryDir,
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
	flags.StringVar(&factoryDir, "factory-repo", "", "path to the factory checkout")
	flags.StringVar(&configBranch, "config-branch", "main", "branch that holds config")
	flags.StringVar(&stateBranch, "state-branch", "state", "branch that holds state")
	return
}
