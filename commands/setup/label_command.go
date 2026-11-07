package setup

import (
	"fmt"
	"io"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/shared"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func newSetupLabel() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "label",
		Short: "Add or list labels",
	}
	cmd.AddCommand(newAddLabel())
	cmd.AddCommand(newListLabels())
	return
}

func newAddLabel() (cmd *cobra.Command) {
	var owner string
	var repo string
	var name string
	var color string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Store a label",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAddLabel(cmd, owner, repo, name, color)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&name, "name", "", "label name")
	flags.StringVar(&color, "color", "", "six hex digits")
	shared.RequireFlag(cmd, "owner")
	shared.RequireFlag(cmd, "repo")
	shared.RequireFlag(cmd, "name")
	shared.RequireFlag(cmd, "color")
	return
}

func newListLabels() (cmd *cobra.Command) {
	var owner string
	var repo string
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print stored labels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runListLabels(cmd, owner, repo)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	shared.RequireFlag(cmd, "owner")
	shared.RequireFlag(cmd, "repo")
	return
}

func runAddLabel(cmd *cobra.Command, owner string, repo string, name string, color string) (err error) {
	ctx := cmd.Context()
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		_, useErr = addLabel(ctx, db, name, color)
		return
	}
	err = shared.WithRepo(ctx, owner, repo, use)
	return
}

func runListLabels(cmd *cobra.Command, owner string, repo string) (err error) {
	ctx := cmd.Context()
	var labels []configrecords.Label
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		labels, useErr = db.Queries.ListLabels(ctx)
		if useErr != nil {
			log.Error("list labels", "err", useErr)
		}
		return
	}
	err = shared.WithRepo(ctx, owner, repo, use)
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
