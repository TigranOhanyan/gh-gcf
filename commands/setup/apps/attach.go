package apps

import (
	"fmt"
	"strconv"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/setup"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func newAttach() (cmd *cobra.Command) {
	var owner string
	var repo string
	var id string
	cmd = &cobra.Command{
		Use:   "attach",
		Short: "Attach an App to an app repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAttach(cmd, owner, repo, id)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&id, "id", "", "app id")
	setup.RequireFlag(cmd, "owner")
	setup.RequireFlag(cmd, "repo")
	setup.RequireFlag(cmd, "id")
	return
}

func runAttach(cmd *cobra.Command, owner string, repo string, rawID string) (err error) {
	ctx := cmd.Context()
	use := func(dir string, _ string, _ string, db *configdb.DB) (useErr error) {
		id, useErr := setup.ParseAppID(rawID)
		if useErr != nil {
			return
		}
		useErr = requireRegistered(dir, id)
		if useErr != nil {
			return
		}
		useErr = setup.AttachApp(ctx, db, id)
		return
	}
	err = setup.WithRepo(ctx, owner, repo, use)
	return
}

func requireRegistered(factoryDir string, id int64) (err error) {
	ids, err := commands.ListAppIDs(factoryDir)
	if err != nil {
		return
	}
	for _, existing := range ids {
		same := existing == id
		if same {
			return
		}
	}
	text := strconv.FormatInt(id, 10)
	err = fmt.Errorf("app %s is not registered", text)
	log.Info("app is not registered", "app", id)
	return
}
