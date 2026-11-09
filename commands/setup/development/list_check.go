package development

import (
	"fmt"
	"io"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func newListCheck() (cmd *cobra.Command) {
	var owner string
	var repo string
	var development string
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print check names",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runListCheck(cmd, owner, repo, development)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&development, "development", "", "development name")
	common.RequireFlag(cmd, "owner")
	common.RequireFlag(cmd, "repo")
	common.RequireFlag(cmd, "development")
	return
}

func runListCheck(cmd *cobra.Command, owner string, repo string, rawDevelopment string) (err error) {
	ctx := cmd.Context()
	var names []string
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		developmentName, useErr := common.NormalizeSegment(rawDevelopment)
		if useErr != nil {
			return
		}
		names, useErr = db.Queries.ListDevelopmentChecks(ctx, developmentName)
		if useErr != nil {
			log.Error("list development checks", "development", developmentName, "err", useErr)
		}
		return
	}
	err = common.WithRepo(ctx, owner, repo, use)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printChecks(writer, names)
	return
}

func printChecks(writer io.Writer, names []string) (err error) {
	for _, name := range names {
		_, err = fmt.Fprintln(writer, name)
		if err != nil {
			return
		}
	}
	return
}
