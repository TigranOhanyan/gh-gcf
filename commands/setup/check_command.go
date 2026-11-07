package setup

import (
	"fmt"
	"io"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/shared"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func newSetupCheck() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "check",
		Short: "Add or list checks",
	}
	cmd.AddCommand(newAddCheck())
	cmd.AddCommand(newListChecks())
	return
}

func newAddCheck() (cmd *cobra.Command) {
	var owner string
	var repo string
	var builder string
	var name string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Store a check",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAddCheck(cmd, owner, repo, builder, name)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&builder, "builder", "", "builder name")
	flags.StringVar(&name, "name", "", "check name")
	shared.RequireFlag(cmd, "owner")
	shared.RequireFlag(cmd, "repo")
	shared.RequireFlag(cmd, "builder")
	shared.RequireFlag(cmd, "name")
	return
}

func newListChecks() (cmd *cobra.Command) {
	var owner string
	var repo string
	var builder string
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print checks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runListChecks(cmd, owner, repo, builder)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&builder, "builder", "", "builder name")
	shared.RequireFlag(cmd, "owner")
	shared.RequireFlag(cmd, "repo")
	shared.RequireFlag(cmd, "builder")
	return
}

func runAddCheck(cmd *cobra.Command, owner string, repo string, builder string, name string) (err error) {
	ctx := cmd.Context()
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		_, useErr = addGithubCheck(ctx, db, builder, name)
		return
	}
	err = shared.WithRepo(ctx, owner, repo, use)
	return
}

func runListChecks(cmd *cobra.Command, owner string, repo string, builder string) (err error) {
	ctx := cmd.Context()
	var names []string
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		names, useErr = db.Queries.ListGithubChecks(ctx, builder)
		if useErr != nil {
			log.Error("list checks", "err", useErr)
		}
		return
	}
	err = shared.WithRepo(ctx, owner, repo, use)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printNames(writer, names)
	return
}

func printNames(writer io.Writer, names []string) (err error) {
	for _, name := range names {
		_, err = fmt.Fprintln(writer, name)
		if err != nil {
			return
		}
	}
	return
}
