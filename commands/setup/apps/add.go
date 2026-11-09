package apps

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
)

func newAdd() (cmd *cobra.Command) {
	var id string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Register an App",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAdd(cmd, id)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&id, "id", "", "app id")
	common.RequireFlag(cmd, "id")
	return
}

func runAdd(cmd *cobra.Command, rawID string) (err error) {
	ctx := cmd.Context()
	dir, err := common.CurrentFactory(ctx)
	if err != nil {
		return
	}
	_, err = CreateFile(dir, rawID)
	return
}
