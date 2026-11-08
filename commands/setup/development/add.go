package development

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/config"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func newAdd() (cmd *cobra.Command) {
	var owner string
	var repo string
	var name string
	var starting string
	var attention string
	var standardsApp string
	var specificationApp string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Store a development",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAdd(cmd, owner, repo, name, starting, attention, standardsApp, specificationApp)
			return
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&owner, "owner", "", "owner of the app repo")
	flags.StringVar(&repo, "repo", "", "name of the app repo")
	flags.StringVar(&name, "name", "", "development name")
	flags.StringVar(&starting, "starting", "", "starting label")
	flags.StringVar(&attention, "attention", "", "attention label")
	flags.StringVar(&standardsApp, "standards-app", "", "standards reviewer app id")
	flags.StringVar(&specificationApp, "specification-app", "", "specification reviewer app id")
	config.RequireFlag(cmd, "owner")
	config.RequireFlag(cmd, "repo")
	config.RequireFlag(cmd, "name")
	config.RequireFlag(cmd, "starting")
	config.RequireFlag(cmd, "attention")
	config.RequireFlag(cmd, "standards-app")
	config.RequireFlag(cmd, "specification-app")
	return
}

func runAdd(cmd *cobra.Command, rawOwner string, rawRepo string, rawName string, rawStarting string, rawAttention string, rawStandardsApp string, rawSpecificationApp string) (err error) {
	ctx := cmd.Context()
	use := func(_ string, _ string, _ string, db *configdb.DB) (useErr error) {
		starting, useErr := config.NormalizeLabelName(rawStarting)
		if useErr != nil {
			return
		}
		attention, useErr := config.NormalizeLabelName(rawAttention)
		if useErr != nil {
			return
		}
		standardsApp, useErr := config.ParseAppID(rawStandardsApp)
		if useErr != nil {
			return
		}
		specificationApp, useErr := config.ParseAppID(rawSpecificationApp)
		if useErr != nil {
			return
		}
		_, useErr = add(ctx, db, rawName, starting, attention, standardsApp, specificationApp)
		return
	}
	err = config.WithRepo(ctx, rawOwner, rawRepo, use)
	return
}

func add(ctx context.Context, db *configdb.DB, rawName string, starting string, attention string, standardsApp int64, specificationApp int64) (name string, err error) {
	name, err = config.NormalizeSegment(rawName)
	if err != nil {
		return
	}
	sameApp := standardsApp == specificationApp
	if sameApp {
		err = fmt.Errorf("reviewer apps must differ")
		log.Info("reviewer apps", "app", standardsApp)
		return
	}
	err = refuseTakenStart(ctx, db, starting)
	if err != nil {
		return
	}
	err = refuseTakenApp(ctx, db, standardsApp, specificationApp)
	if err != nil {
		return
	}

	params := configrecords.CreateDevelopmentParams{
		Name:                     name,
		StartingLabel:            starting,
		AttentionLabel:           attention,
		StandardsReviewerApp:     standardsApp,
		SpecificationReviewerApp: specificationApp,
	}
	err = db.Queries.CreateDevelopment(ctx, params)
	if err != nil {
		log.Error("create development", "name", name, "err", err)
		return
	}
	log.Info("create development", "name", name)
	return
}

func refuseTakenStart(ctx context.Context, db *configdb.DB, starting string) (err error) {
	labels, err := db.Queries.ListDevelopmentStartingLabels(ctx)
	if err != nil {
		log.Error("list development starting labels", "err", err)
		return
	}
	for _, existing := range labels {
		same := existing == starting
		if same {
			err = fmt.Errorf("starting label %s is already used", starting)
			log.Info("starting label used", "label", starting)
			return
		}
	}
	return
}

func refuseTakenApp(ctx context.Context, db *configdb.DB, standardsApp int64, specificationApp int64) (err error) {
	ids, err := db.Queries.ListDevelopmentReviewerApps(ctx)
	if err != nil {
		log.Error("list development reviewer apps", "err", err)
		return
	}
	for _, existing := range ids {
		standardsTaken := existing == standardsApp
		specificationTaken := existing == specificationApp
		taken := standardsTaken || specificationTaken
		if taken {
			err = fmt.Errorf("app %d is already used", existing)
			log.Info("reviewer app used", "app", existing)
			return
		}
	}
	return
}
