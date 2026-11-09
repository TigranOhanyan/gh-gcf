package label

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func newAdd() (cmd *cobra.Command) {
	var owner string
	var repo string
	var name string
	var color string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Store a label",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAdd(cmd, owner, repo, name, color)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&name, "name", "", "label name")
	flags.StringVar(&color, "color", "", "six hex digits")
	common.RequireFlag(cmd, "owner")
	common.RequireFlag(cmd, "repo")
	common.RequireFlag(cmd, "name")
	common.RequireFlag(cmd, "color")
	return
}

func runAdd(cmd *cobra.Command, owner string, repo string, name string, color string) (err error) {
	ctx := cmd.Context()
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		_, useErr = Add(ctx, db, name, color)
		return
	}
	err = common.WithRepo(ctx, owner, repo, use)
	return
}
