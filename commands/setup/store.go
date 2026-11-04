package setup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

const maxLabelName = 50

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

			configFile := configPath(factoryDir, ownerName, nameText)
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
	owner, err = normalizeSegment(owner)
	if err != nil {
		return
	}
	name, err = normalizeSegment(name)
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

	path := configPath(factoryDir, owner, name)
	db, err = configdb.OpenAndMigrate(ctx, path)
	if err != nil {
		log.Error("create config", "path", path, "err", err)
		return
	}
	log.Info("create repo", "owner", owner, "name", name, "path", path)
	return
}

func openConfig(ctx context.Context, path string) (db *configdb.DB, missing bool, err error) {
	_, err = os.Stat(path)
	missing = os.IsNotExist(err)
	if missing {
		err = nil
		return
	}
	if err != nil {
		log.Error("stat config", "path", path, "err", err)
		return
	}

	db, err = configdb.OpenAndMigrate(ctx, path)
	if err != nil {
		log.Error("open config", "path", path, "err", err)
	}
	return
}

func addLabel(ctx context.Context, db *configdb.DB, rawName string, rawColor string) (name string, err error) {
	name, err = normalizeLabelName(rawName)
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

func listAppIDs(factoryDir string) (ids []int64, err error) {
	dir := filepath.Join(factoryDir, "apps")
	entries, err := os.ReadDir(dir)
	missing := os.IsNotExist(err)
	if missing {
		err = nil
		return
	}
	if err != nil {
		log.Error("list apps", "path", dir, "err", err)
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		id, parseErr := strconv.ParseInt(name, 10, 64)
		canonical := strconv.FormatInt(id, 10)
		isDir := entry.IsDir()
		bad := isDir || parseErr != nil || id <= 0 || canonical != name
		if bad {
			err = fmt.Errorf("app file name is not an app id: %s", name)
			log.Error("list apps", "err", err)
			return
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i int, j int) bool {
		return ids[i] < ids[j]
	})
	return
}

func createAppFile(factoryDir string, raw string) (id int64, err error) {
	text := strings.TrimSpace(raw)
	parsed, parseErr := strconv.ParseInt(text, 10, 64)
	canonical := strconv.FormatInt(parsed, 10)
	bad := parseErr != nil || parsed <= 0 || canonical != text
	if bad {
		err = fmt.Errorf("app id must be a positive integer")
		return
	}

	dir := filepath.Join(factoryDir, "apps")
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Error("create app", "path", dir, "err", err)
		return
	}

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
	id = parsed
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

func configPath(factoryDir string, owner string, name string) (path string) {
	path = filepath.Join(factoryDir, "repos", owner, name, "config.db")
	return
}

func normalizeSegment(raw string) (segment string, err error) {
	segment = strings.TrimSpace(raw)
	empty := segment == ""
	if empty {
		err = fmt.Errorf("name is empty")
		segment = ""
		return
	}

	hasSlash := strings.Contains(segment, "/")
	hasBackslash := strings.Contains(segment, "\\")
	hasSep := hasSlash || hasBackslash
	dot := segment == "." || segment == ".."
	bad := hasSep || dot
	if bad {
		err = fmt.Errorf("name must be a single path segment")
		segment = ""
	}
	return
}

func normalizeLabelName(raw string) (name string, err error) {
	name = strings.TrimSpace(raw)
	empty := name == ""
	if empty {
		err = fmt.Errorf("label name is empty")
		return
	}

	count := utf8.RuneCountInString(name)
	tooLong := count > maxLabelName
	if tooLong {
		err = fmt.Errorf("label name is longer than 50 characters")
	}
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
