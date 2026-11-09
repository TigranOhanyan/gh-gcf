package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

type AppRepo struct {
	Owner     string
	Name      string
	HasConfig bool
}

func List(factoryDir string) (repos []AppRepo, err error) {
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

			configFile := common.ConfigPath(factoryDir, ownerName, nameText)
			_, statErr := os.Stat(configFile)
			hasConfig := statErr == nil
			stored := AppRepo{
				Owner:     ownerName,
				Name:      nameText,
				HasConfig: hasConfig,
			}
			repos = append(repos, stored)
		}
	}

	sort.Slice(repos, func(i int, j int) bool {
		left := Key(repos[i])
		right := Key(repos[j])
		return left < right
	})
	return
}

func Key(stored AppRepo) (key string) {
	key = stored.Owner + "/" + stored.Name
	return
}

func Create(ctx context.Context, factoryDir string, owner string, name string) (db *configdb.DB, err error) {
	owner, err = common.NormalizeSegment(owner)
	if err != nil {
		return
	}
	name, err = common.NormalizeSegment(name)
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

	path := common.ConfigPath(factoryDir, owner, name)
	db, err = configdb.OpenAndMigrate(ctx, path)
	if err != nil {
		log.Error("create config", "path", path, "err", err)
		return
	}
	log.Info("create repo", "owner", owner, "name", name, "path", path)
	return
}
