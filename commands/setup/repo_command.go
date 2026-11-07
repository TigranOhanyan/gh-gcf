package setup

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/shared"
)

func newSetupRepo() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "repo",
		Short: "Add or list app repos",
	}
	cmd.AddCommand(newAddRepo())
	cmd.AddCommand(newListRepos())
	return
}

func newAddRepo() (cmd *cobra.Command) {
	var owner string
	var name string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Store an app repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAddRepo(cmd, owner, name)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&name, "name", "", "name of the app repo")
	shared.RequireFlag(cmd, "owner")
	shared.RequireFlag(cmd, "name")
	return
}

func newListRepos() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "list",
		Short: "Print app repos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runListRepos(cmd)
			return
		},
	}
	return
}

func runAddRepo(cmd *cobra.Command, owner string, name string) (err error) {
	ctx := cmd.Context()
	dir, err := shared.CurrentFactory(ctx)
	if err != nil {
		return
	}
	db, err := createRepo(ctx, dir, owner, name)
	if err != nil {
		return
	}
	shared.CloseConfig(db)
	return
}

func runListRepos(cmd *cobra.Command) (err error) {
	ctx := cmd.Context()
	dir, err := shared.CurrentFactory(ctx)
	if err != nil {
		return
	}
	repos, err := listRepos(dir)
	if err != nil {
		return
	}
	writer := cmd.OutOrStdout()
	err = printRepos(writer, repos)
	return
}

func printRepos(writer io.Writer, repos []appRepo) (err error) {
	for _, repo := range repos {
		line := repoKey(repo)
		_, err = fmt.Fprintln(writer, line)
		if err != nil {
			return
		}
	}
	return
}
