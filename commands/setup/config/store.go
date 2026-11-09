package config

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

	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

var colorHex = regexp.MustCompile(`^[0-9a-f]{6}$`)

type AppRepo struct {
	Owner     string
	Name      string
	HasConfig bool
}

func ListRepos(factoryDir string) (repos []AppRepo, err error) {
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

			configFile := ConfigPath(factoryDir, ownerName, nameText)
			_, statErr := os.Stat(configFile)
			hasConfig := statErr == nil
			repo := AppRepo{
				Owner:     ownerName,
				Name:      nameText,
				HasConfig: hasConfig,
			}
			repos = append(repos, repo)
		}
	}

	sort.Slice(repos, func(i int, j int) bool {
		left := RepoKey(repos[i])
		right := RepoKey(repos[j])
		return left < right
	})
	return
}

func RepoKey(repo AppRepo) (key string) {
	key = repo.Owner + "/" + repo.Name
	return
}

func CreateRepo(ctx context.Context, factoryDir string, owner string, name string) (db *configdb.DB, err error) {
	owner, err = NormalizeSegment(owner)
	if err != nil {
		return
	}
	name, err = NormalizeSegment(name)
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

	path := ConfigPath(factoryDir, owner, name)
	db, err = configdb.OpenAndMigrate(ctx, path)
	if err != nil {
		log.Error("create config", "path", path, "err", err)
		return
	}
	log.Info("create repo", "owner", owner, "name", name, "path", path)
	return
}

func AddLabel(ctx context.Context, db *configdb.DB, rawName string, rawColor string) (name string, err error) {
	name, err = NormalizeLabelName(rawName)
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

func CreateAppFile(factoryDir string, raw string) (id int64, err error) {
	id, err = ParseAppID(raw)
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

func AttachApp(ctx context.Context, db *configdb.DB, id int64) (err error) {
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
