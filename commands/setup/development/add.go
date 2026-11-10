package development

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

var instructionRoles = []string{
	"builder",
	"standards_reviewer",
	"specification_reviewer",
	"fixer",
}

func newAdd() (cmd *cobra.Command) {
	var owner string
	var repo string
	var name string
	var starting string
	var attention string
	var standardsApp string
	var specificationApp string
	var builderApp string
	var fixerApp string
	cmd = &cobra.Command{
		Use:   "add",
		Short: "Store a development",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			err = runAdd(cmd, owner, repo, name, starting, attention, standardsApp, specificationApp, builderApp, fixerApp)
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
	flags.StringVar(&builderApp, "builder-app", "", "builder app id")
	flags.StringVar(&fixerApp, "fixer-app", "", "fixer app id")
	common.RequireFlag(cmd, "owner")
	common.RequireFlag(cmd, "repo")
	common.RequireFlag(cmd, "name")
	common.RequireFlag(cmd, "starting")
	common.RequireFlag(cmd, "attention")
	common.RequireFlag(cmd, "standards-app")
	common.RequireFlag(cmd, "specification-app")
	common.RequireFlag(cmd, "builder-app")
	common.RequireFlag(cmd, "fixer-app")
	return
}

func runAdd(cmd *cobra.Command, rawOwner string, rawRepo string, rawName string, rawStarting string, rawAttention string, rawStandardsApp string, rawSpecificationApp string, rawBuilderApp string, rawFixerApp string) (err error) {
	ctx := cmd.Context()
	use := func(dir string, owner string, repo string, db *configdb.DB) (useErr error) {
		repoDir := filepath.Join(dir, "repos", owner, repo)
		_, useErr = Add(ctx, db, repoDir, rawName, rawStarting, rawAttention, rawStandardsApp, rawSpecificationApp, rawBuilderApp, rawFixerApp)
		return
	}
	err = common.WithRepo(ctx, rawOwner, rawRepo, use)
	return
}

func Add(ctx context.Context, db *configdb.DB, repoDir string, rawName string, rawStarting string, rawAttention string, rawStandardsApp string, rawSpecificationApp string, rawBuilderApp string, rawFixerApp string) (name string, err error) {
	name, err = common.NormalizeSegment(rawName)
	if err != nil {
		return
	}
	starting, err := common.NormalizeLabelName(rawStarting)
	if err != nil {
		return
	}
	attention, err := common.NormalizeLabelName(rawAttention)
	if err != nil {
		return
	}
	standardsApp, err := common.ParseAppID(rawStandardsApp)
	if err != nil {
		return
	}
	specificationApp, err := common.ParseAppID(rawSpecificationApp)
	if err != nil {
		return
	}
	builderApp, err := common.ParseAppID(rawBuilderApp)
	if err != nil {
		return
	}
	fixerApp, err := common.ParseAppID(rawFixerApp)
	if err != nil {
		return
	}
	sameApp := standardsApp == specificationApp
	if sameApp {
		err = fmt.Errorf("reviewer apps must differ")
		log.Info("reviewer apps", "app", standardsApp)
		return
	}
	err = refuseReviewerRole("builder", builderApp, standardsApp, specificationApp)
	if err != nil {
		return
	}
	err = refuseReviewerRole("fixer", fixerApp, standardsApp, specificationApp)
	if err != nil {
		return
	}
	err = refuseUnregistered(ctx, db, builderApp)
	if err != nil {
		return
	}
	err = refuseUnregistered(ctx, db, fixerApp)
	if err != nil {
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
	err = refuseExistingFolder(repoDir, name)
	if err != nil {
		return
	}

	params := configrecords.CreateDevelopmentParams{
		Name:                     name,
		StartingLabel:            starting,
		AttentionLabel:           attention,
		StandardsReviewerApp:     standardsApp,
		SpecificationReviewerApp: specificationApp,
		BuilderApp:               builderApp,
		FixerApp:                 fixerApp,
	}
	err = db.Queries.CreateDevelopment(ctx, params)
	if err != nil {
		log.Error("create development", "name", name, "err", err)
		return
	}
	err = writeInstructions(repoDir, name)
	if err != nil {
		return
	}
	log.Info("create development", "name", name)
	return
}

func refuseExistingFolder(repoDir string, name string) (err error) {
	dir := filepath.Join(repoDir, "developments", name)
	_, statErr := os.Stat(dir)
	exists := statErr == nil
	if exists {
		err = fmt.Errorf("%s should be removed", dir)
		log.Info("development folder exists", "path", dir)
		return
	}
	missing := os.IsNotExist(statErr)
	if !missing {
		err = statErr
		log.Error("stat development folder", "path", dir, "err", err)
	}
	return
}

func writeInstructions(repoDir string, name string) (err error) {
	dir := filepath.Join(repoDir, "developments", name)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Error("create instructions", "path", dir, "err", err)
		return
	}
	empty := []byte{}
	for _, role := range instructionRoles {
		fileName := role + ".md"
		path := filepath.Join(dir, fileName)
		err = os.WriteFile(path, empty, 0644)
		if err != nil {
			log.Error("create instruction", "path", path, "err", err)
			return
		}
	}
	log.Info("create instructions", "development", name, "path", dir)
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

func refuseReviewerRole(role string, app int64, standardsApp int64, specificationApp int64) (err error) {
	matchesStandards := app == standardsApp
	matchesSpecification := app == specificationApp
	reviewer := matchesStandards || matchesSpecification
	if !reviewer {
		return
	}
	err = fmt.Errorf("%s app must differ from the reviewer apps", role)
	log.Info("app is a reviewer", "role", role, "app", app)
	return
}

func refuseUnregistered(ctx context.Context, db *configdb.DB, app int64) (err error) {
	ids, err := db.Queries.ListGithubApps(ctx)
	if err != nil {
		log.Error("list apps", "err", err)
		return
	}
	for _, existing := range ids {
		same := existing == app
		if same {
			return
		}
	}
	text := strconv.FormatInt(app, 10)
	err = fmt.Errorf("app %s is not registered", text)
	log.Info("app is not registered", "app", app)
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
