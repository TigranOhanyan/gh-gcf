package setup

import (
	"charm.land/log/v2"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/shared"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

var colorHex = regexp.MustCompile(`^[0-9a-f]{6}$`)

type appRepo struct {
	owner     string
	name      string
	hasConfig bool
}

func listRepos(factoryDir string) (repos []appRepo, err error) {
	reposDir := filepath.Join(factoryDir, "repos")
	owners, err := os.ReadDir(reposDir)
	missing := os.IsNotExist(err)
	if missing {
		err = nil
		return
	}
	if err != nil {
		log.Error("list repos", "path", reposDir, "err", err)
		return
	}

	for _, owner := range owners {
		ownerName := owner.Name()
		ownerIsDir := owner.IsDir()
		if !ownerIsDir {
			err = fmt.Errorf("path is not repos/<owner>/<name>: repos/%s", ownerName)
			log.Error("list repos", "err", err)
			return
		}

		ownerDir := filepath.Join(reposDir, ownerName)
		names, readErr := os.ReadDir(ownerDir)
		if readErr != nil {
			err = readErr
			log.Error("list repos", "path", ownerDir, "err", err)
			return
		}
		for _, repoName := range names {
			nameText := repoName.Name()
			nameIsDir := repoName.IsDir()
			if !nameIsDir {
				err = fmt.Errorf("path is not repos/<owner>/<name>: repos/%s/%s", ownerName, nameText)
				log.Error("list repos", "err", err)
				return
			}

			configFile := shared.ConfigPath(factoryDir, ownerName, nameText)
			_, statErr := os.Stat(configFile)
			hasConfig := statErr == nil
			repo := appRepo{
				owner:     ownerName,
				name:      nameText,
				hasConfig: hasConfig,
			}
			repos = append(repos, repo)
		}
	}

	sort.Slice(repos, func(i int, j int) bool {
		left := repoKey(repos[i])
		right := repoKey(repos[j])
		return left < right
	})
	return
}

func repoKey(repo appRepo) (key string) {
	key = repo.owner + "/" + repo.name
	return
}

func createRepo(ctx context.Context, factoryDir string, owner string, name string) (db *configdb.DB, err error) {
	owner, err = shared.NormalizeSegment(owner)
	if err != nil {
		return
	}
	name, err = shared.NormalizeSegment(name)
	if err != nil {
		return
	}

	dir := filepath.Join(factoryDir, "repos", owner, name)
	_, err = os.Stat(dir)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		log.Error("stat repo", "path", dir, "err", err)
		return
	}
	if !missing {
		err = fmt.Errorf("repo %s/%s already exists", owner, name)
		log.Info("repo exists", "owner", owner, "name", name)
		return
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Error("create repo", "path", dir, "err", err)
		return
	}

	path := shared.ConfigPath(factoryDir, owner, name)
	db, err = configdb.OpenAndMigrate(ctx, path)
	if err != nil {
		log.Error("create config", "path", path, "err", err)
		return
	}
	log.Info("create repo", "owner", owner, "name", name, "path", path)
	return
}

func addLabel(ctx context.Context, db *configdb.DB, rawName string, rawColor string) (name string, err error) {
	name, err = shared.NormalizeLabelName(rawName)
	if err != nil {
		return
	}
	color, err := normalizeColor(rawColor)
	if err != nil {
		return
	}

	labels, err := db.Queries.ListLabels(ctx)
	if err != nil {
		log.Error("list labels", "err", err)
		return
	}
	for _, existing := range labels {
		same := existing.Name == name
		if same {
			err = fmt.Errorf("label %s already exists", name)
			log.Info("label exists", "name", name)
			return
		}
	}

	params := configrecords.CreateLabelParams{
		Name:  name,
		Color: color,
	}
	err = db.Queries.CreateLabel(ctx, params)
	if err != nil {
		log.Error("create label", "name", name, "err", err)
		return
	}
	log.Info("create label", "name", name, "color", color)
	return
}

func createAppFile(factoryDir string, raw string) (id int64, err error) {
	id, err = shared.ParseAppID(raw)
	if err != nil {
		return
	}

	dir := filepath.Join(factoryDir, "apps")
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Error("create app", "path", dir, "err", err)
		return
	}

	canonical := strconv.FormatInt(id, 10)
	path := filepath.Join(dir, canonical)
	_, statErr := os.Stat(path)
	exists := statErr == nil
	if exists {
		err = fmt.Errorf("app %s already exists", canonical)
		log.Info("app exists", "app", canonical)
		return
	}
	missing := os.IsNotExist(statErr)
	if statErr != nil && !missing {
		err = statErr
		log.Error("stat app", "path", path, "err", err)
		return
	}

	empty := []byte{}
	err = os.WriteFile(path, empty, 0644)
	if err != nil {
		log.Error("create app", "path", path, "err", err)
		return
	}
	log.Info("create app", "app", id, "path", path)
	return
}

func attachApp(ctx context.Context, db *configdb.DB, id int64) (err error) {
	ids, err := db.Queries.ListGithubApps(ctx)
	if err != nil {
		log.Error("list apps", "err", err)
		return
	}
	for _, existing := range ids {
		same := existing == id
		if same {
			err = fmt.Errorf("app %d already exists", id)
			log.Info("app exists", "app", id)
			return
		}
	}

	err = db.Queries.CreateGithubApp(ctx, id)
	if err != nil {
		log.Error("attach app", "app", id, "err", err)
		return
	}
	log.Info("attach app", "app", id)
	return
}

func secretName(id int64) (name string) {
	name = fmt.Sprintf("GITHUB_APP_%d", id)
	return
}

func normalizeColor(raw string) (color string, err error) {
	color = strings.TrimSpace(raw)
	color = strings.TrimPrefix(color, "#")
	color = strings.ToLower(color)
	matched := colorHex.MatchString(color)
	if !matched {
		err = fmt.Errorf("color must be six hex digits")
		color = ""
	}
	return
}

const roleBuilder = "builder"
const roleReviewer = "reviewer"
const roleFixer = "fixer"

func addBuilderMember(ctx context.Context, db *configdb.DB, factoryDir string, owner string, repo string, rawName string, starting string, attention string) (name string, err error) {
	name, err = normalizeMemberName(rawName)
	if err != nil {
		return
	}
	sameLabel := starting == attention
	if sameLabel {
		err = fmt.Errorf("starting label and attention label must differ")
		return
	}
	err = refuseTakenMember(ctx, db, name)
	if err != nil {
		return
	}
	err = refuseTakenStart(ctx, db, starting)
	if err != nil {
		return
	}

	path := memberInstructionPath(factoryDir, owner, repo, name)
	err = writeMemberFile(path)
	if err != nil {
		return
	}
	child := func(q *configrecords.Queries) (writeErr error) {
		params := configrecords.CreateBuilderMemberParams{
			Name:          name,
			StartingLabel: starting,
		}
		writeErr = q.CreateBuilderMember(ctx, params)
		return
	}
	err = insertMember(ctx, db, name, roleBuilder, attention, child)
	if err != nil {
		removeErr := os.Remove(path)
		if removeErr != nil {
			log.Error("remove instruction", "path", path, "err", removeErr)
		}
	}
	return
}

func addReviewerMember(ctx context.Context, db *configdb.DB, factoryDir string, owner string, repo string, builder string, rawName string, appID int64, attention string) (name string, err error) {
	name, err = normalizeMemberName(rawName)
	if err != nil {
		return
	}
	err = refuseTakenMember(ctx, db, name)
	if err != nil {
		return
	}
	err = refuseTakenApp(ctx, db, appID)
	if err != nil {
		return
	}

	path := memberInstructionPath(factoryDir, owner, repo, name)
	err = writeMemberFile(path)
	if err != nil {
		return
	}
	child := func(q *configrecords.Queries) (writeErr error) {
		params := configrecords.CreateReviewerMemberParams{
			Name:        name,
			BuilderName: builder,
			GithubApp:   appID,
		}
		writeErr = q.CreateReviewerMember(ctx, params)
		return
	}
	err = insertMember(ctx, db, name, roleReviewer, attention, child)
	if err != nil {
		removeErr := os.Remove(path)
		if removeErr != nil {
			log.Error("remove instruction", "path", path, "err", removeErr)
		}
	}
	return
}

func addFixerMember(ctx context.Context, db *configdb.DB, factoryDir string, owner string, repo string, builder string, rawName string, attention string) (name string, err error) {
	name, err = normalizeMemberName(rawName)
	if err != nil {
		return
	}
	err = refuseTakenMember(ctx, db, name)
	if err != nil {
		return
	}
	err = refuseExistingFixer(ctx, db, builder)
	if err != nil {
		return
	}

	path := memberInstructionPath(factoryDir, owner, repo, name)
	err = writeMemberFile(path)
	if err != nil {
		return
	}
	child := func(q *configrecords.Queries) (writeErr error) {
		params := configrecords.CreateFixerMemberParams{
			Name:        name,
			BuilderName: builder,
		}
		writeErr = q.CreateFixerMember(ctx, params)
		return
	}
	err = insertMember(ctx, db, name, roleFixer, attention, child)
	if err != nil {
		removeErr := os.Remove(path)
		if removeErr != nil {
			log.Error("remove instruction", "path", path, "err", removeErr)
		}
	}
	return
}

func addGithubCheck(ctx context.Context, db *configdb.DB, builder string, rawName string) (name string, err error) {
	name, err = shared.NormalizeCheckName(rawName)
	if err != nil {
		return
	}
	names, err := db.Queries.ListGithubChecks(ctx, builder)
	if err != nil {
		log.Error("list checks", "err", err)
		return
	}
	for _, existing := range names {
		same := existing == name
		if same {
			err = fmt.Errorf("check %s already exists", name)
			log.Info("check exists", "name", name, "builder", builder)
			return
		}
	}
	params := configrecords.CreateGithubCheckParams{
		BuilderName: builder,
		Name:        name,
	}
	err = db.Queries.CreateGithubCheck(ctx, params)
	if err != nil {
		log.Error("create check", "name", name, "builder", builder, "err", err)
		return
	}
	log.Info("create check", "name", name, "builder", builder)
	return
}

func insertMember(ctx context.Context, db *configdb.DB, name string, role string, attention string, child func(q *configrecords.Queries) error) (err error) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin member", "name", name, "err", err)
		return
	}
	defer func() {
		failed := err != nil
		if failed {
			_ = tx.Rollback()
		}
	}()

	queries := db.Queries.WithTx(tx)
	member := configrecords.CreateMemberParams{
		Name:           name,
		Role:           role,
		AttentionLabel: attention,
	}
	err = queries.CreateMember(ctx, member)
	if err != nil {
		log.Error("create member", "name", name, "role", role, "err", err)
		return
	}
	err = child(queries)
	if err != nil {
		log.Error("create member", "name", name, "role", role, "err", err)
		return
	}
	err = tx.Commit()
	if err != nil {
		log.Error("commit member", "name", name, "err", err)
	}
	return
}

func refuseTakenMember(ctx context.Context, db *configdb.DB, name string) (err error) {
	names, err := db.Queries.ListMemberNames(ctx)
	if err != nil {
		log.Error("list members", "err", err)
		return
	}
	for _, existing := range names {
		same := existing == name
		if same {
			err = fmt.Errorf("member %s already exists", name)
			log.Info("member exists", "name", name)
			return
		}
	}
	return
}

func refuseTakenStart(ctx context.Context, db *configdb.DB, starting string) (err error) {
	labels, err := db.Queries.ListStartingLabels(ctx)
	if err != nil {
		log.Error("list starting labels", "err", err)
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

func refuseTakenApp(ctx context.Context, db *configdb.DB, appID int64) (err error) {
	ids, err := db.Queries.ListReviewerApps(ctx)
	if err != nil {
		log.Error("list reviewer apps", "err", err)
		return
	}
	for _, existing := range ids {
		same := existing == appID
		if same {
			err = fmt.Errorf("app %d is already used", appID)
			log.Info("reviewer app used", "app", appID)
			return
		}
	}
	return
}

func refuseExistingFixer(ctx context.Context, db *configdb.DB, builder string) (err error) {
	rows, err := db.Queries.ListFixerMembers(ctx, builder)
	if err != nil {
		log.Error("list fixers", "err", err)
		return
	}
	has := len(rows) > 0
	if has {
		err = fmt.Errorf("fixer already exists")
		log.Info("fixer exists", "builder", builder)
	}
	return
}

func memberInstructionPath(factoryDir string, owner string, repo string, name string) (path string) {
	path = filepath.Join(factoryDir, "repos", owner, repo, "members", name+".md")
	return
}

func writeMemberFile(path string) (err error) {
	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Error("create instruction", "path", dir, "err", err)
		return
	}
	_, statErr := os.Stat(path)
	exists := statErr == nil
	if exists {
		err = fmt.Errorf("instruction %s already exists", filepath.Base(path))
		log.Info("instruction exists", "path", path)
		return
	}
	missing := os.IsNotExist(statErr)
	if statErr != nil && !missing {
		err = statErr
		log.Error("stat instruction", "path", path, "err", err)
		return
	}
	empty := []byte{}
	err = os.WriteFile(path, empty, 0644)
	if err != nil {
		log.Error("create instruction", "path", path, "err", err)
		return
	}
	log.Info("create instruction", "path", path)
	return
}

func normalizeMemberName(raw string) (name string, err error) {
	name, err = shared.NormalizeSegment(raw)
	return
}
