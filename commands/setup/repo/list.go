package repo

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/config"
)

func newList() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print app repos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runList(cmd)
			return
		},
	}
	return
}

func runList(cmd *cobra.Command) (err error) {
	ctx := cmd.Context()
	dir, err := config.CurrentFactory(ctx)
	if err != nil {
		return
	}
	repos, err := config.ListRepos(dir)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printRepos(writer, repos)
	return
}

func printRepos(writer io.Writer, repos []config.AppRepo) (err error) {
	for _, repo := range repos {
		line := config.RepoKey(repo)
		_, err = fmt.Fprintln(writer, line)
		if err != nil {
			return
		}
	}
	return
}
