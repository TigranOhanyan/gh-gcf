package repo

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
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
	dir, err := common.CurrentFactory(ctx)
	if err != nil {
		return
	}
	repos, err := List(dir)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printRepos(writer, repos)
	return
}

func printRepos(writer io.Writer, repos []AppRepo) (err error) {
	for _, stored := range repos {
		line := Key(stored)
		_, err = fmt.Fprintln(writer, line)
		if err != nil {
			return
		}
	}
	return
}
