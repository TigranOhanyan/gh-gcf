package development

import (
	"fmt"
	"io"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func newList() (cmd *cobra.Command) {
	var owner string
	var repo string
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print developments",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runList(cmd, owner, repo)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	common.RequireFlag(cmd, "owner")
	common.RequireFlag(cmd, "repo")
	return
}

func runList(cmd *cobra.Command, owner string, repo string) (err error) {
	ctx := cmd.Context()
	var rows []configrecords.Development
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		rows, useErr = db.Queries.ListDevelopments(ctx)
		if useErr != nil {
			log.Error("list developments", "err", useErr)
		}
		return
	}
	err = common.WithRepo(ctx, owner, repo, use)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printDevelopments(writer, rows)
	return
}

func printDevelopments(writer io.Writer, rows []configrecords.Development) (err error) {
	for _, row := range rows {
		line := Line(row)
		_, err = fmt.Fprintln(writer, line)
		if err != nil {
			return
		}
	}
	return
}
