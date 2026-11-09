package repo

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
)

func newAdd() (cmd *cobra.Command) {
	var owner string
	var name string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Store an app repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAdd(cmd, owner, name)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&name, "name", "", "name of the app repo")
	common.RequireFlag(cmd, "owner")
	common.RequireFlag(cmd, "name")
	return
}

func runAdd(cmd *cobra.Command, owner string, name string) (err error) {
	ctx := cmd.Context()
	dir, err := common.CurrentFactory(ctx)
	if err != nil {
		return
	}
	db, err := Create(ctx, dir, owner, name)
	if err != nil {
		return
	}
	common.CloseConfig(db)
	return
}
