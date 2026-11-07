package development

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func newCheck() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "check",
		Short: "Store a check name",
	}
	cmd.AddCommand(newAddCheck())
	return
}

func newAddCheck() (cmd *cobra.Command) {
	var owner string
	var repo string
	var development string
	var name string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Store a check name on a development",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAddCheck(cmd, owner, repo, development, name)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&development, "development", "", "development name")
	flags.StringVar(&name, "name", "", "check name")
	setup.RequireFlag(cmd, "owner")
	setup.RequireFlag(cmd, "repo")
	setup.RequireFlag(cmd, "development")
	setup.RequireFlag(cmd, "name")
	return
}

func runAddCheck(cmd *cobra.Command, rawOwner string, rawRepo string, rawDevelopment string, rawName string) (err error) {
	ctx := cmd.Context()
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		_, useErr = addCheck(ctx, db, rawDevelopment, rawName)
		return
	}
	err = setup.WithRepo(ctx, rawOwner, rawRepo, use)
	return
}

func addCheck(ctx context.Context, db *configdb.DB, rawDevelopment string, rawName string) (name string, err error) {
	development, err := setup.NormalizeSegment(rawDevelopment)
	if err != nil {
		return
	}
	name, err = setup.NormalizeCheckName(rawName)
	if err != nil {
		return
	}
	names, err := db.Queries.ListDevelopmentChecks(ctx, development)
	if err != nil {
		log.Error("list development checks", "development", development, "err", err)
		return
	}
	for _, existing := range names {
		same := existing == name
		if same {
			err = fmt.Errorf("check %s already exists", name)
			log.Info("check exists", "name", name, "development", development)
			return
		}
	}
	params := configrecords.CreateDevelopmentCheckParams{
		DevelopmentName: development,
		Name:            name,
	}
	err = db.Queries.CreateDevelopmentCheck(ctx, params)
	if err != nil {
		log.Error("create development check", "name", name, "development", development, "err", err)
		return
	}
	log.Info("create development check", "name", name, "development", development)
	return
}
