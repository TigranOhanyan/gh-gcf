package label

import (
	"fmt"
	"io"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func newList() (cmd *cobra.Command) {
	var owner string
	var repo string
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print stored labels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runList(cmd, owner, repo)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	setup.RequireFlag(cmd, "owner")
	setup.RequireFlag(cmd, "repo")
	return
}

func runList(cmd *cobra.Command, owner string, repo string) (err error) {
	ctx := cmd.Context()
	var labels []configrecords.Label
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		labels, useErr = db.Queries.ListLabels(ctx)
		if useErr != nil {
			log.Error("list labels", "err", useErr)
		}
		return
	}
	err = setup.WithRepo(ctx, owner, repo, use)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printLabels(writer, labels)
	return
}

func printLabels(writer io.Writer, labels []configrecords.Label) (err error) {
	for _, label := range labels {
		_, err = fmt.Fprintf(writer, "%s  %s\n", label.Name, label.Color)
		if err != nil {
			return
		}
	}
	return
}
