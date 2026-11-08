package apps

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/setup"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func newList() (cmd *cobra.Command) {
	var owner string
	var repo string
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print Apps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runList(cmd, owner, repo)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	return
}

func runList(cmd *cobra.Command, owner string, repo string) (err error) {
	ctx := cmd.Context()
	dir, err := setup.CurrentFactory(ctx)
	if err != nil {
		return
	}
	ownerText := strings.TrimSpace(owner)
	repoText := strings.TrimSpace(repo)
	ownerSet := ownerText != ""
	repoSet := repoText != ""
	neither := !ownerSet && !repoSet
	if neither {
		err = printRegisteredApps(cmd, dir)
		return
	}
	both := ownerSet && repoSet
	if !both {
		err = fmt.Errorf("owner and repo must both be set")
		return
	}
	err = printAttachedApps(cmd, ownerText, repoText)
	return
}

func printRegisteredApps(cmd *cobra.Command, dir string) (err error) {
	ids, err := commands.ListAppIDs(dir)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printAppIDs(writer, ids)
	return
}

func printAttachedApps(cmd *cobra.Command, owner string, repo string) (err error) {
	ctx := cmd.Context()
	var ids []int64
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		ids, useErr = db.Queries.ListGithubApps(ctx)
		if useErr != nil {
			log.Error("list apps", "err", useErr)
		}
		return
	}
	err = setup.WithRepo(ctx, owner, repo, use)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printAppIDs(writer, ids)
	return
}

func printAppIDs(writer io.Writer, ids []int64) (err error) {
	for _, id := range ids {
		text := strconv.FormatInt(id, 10)
		_, err = fmt.Fprintln(writer, text)
		if err != nil {
			return
		}
	}
	return
}
