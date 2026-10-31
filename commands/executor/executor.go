package executor

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	stateentities "github.com/TigranOhanyan/gh-gcf/statedb/entities"
)

// Repo is one app repo with config and state open.
// Owner and Name are the repos/<owner>/<name> folder.
type Repo struct {
	Owner  string
	Name   string
	Config *configdb.DB
	State  *statedb.DB
}

// Action is the work one command does for a single app repo.
// Name is the action name in the state commit subject.
// Apply returns the message for that subject, formed from what the action did.
type Action interface {
	Name() (name string)
	Apply(ctx context.Context, repo Repo, tick stateentities.Tick) (message string, err error)
}

// Executor carries one action across every app repo in FactoryDir.
// ConfigBranch and StateBranch are the branches it opens.
type Executor struct {
	FactoryDir   string
	ConfigBranch string
	StateBranch  string
}

type repoFolder struct {
	owner string
	name  string
	rel   string
}

// Execute opens every app repo, inserts one tick, applies action, and commits state.db.
// Config is opened from ConfigBranch and is not committed. State is opened from StateBranch.
// The commit subject is [tick id] [action name] [owner/name] and the message Apply returned.
// The first action error stops the walk. Repos already committed stay committed.
func (executor Executor) Execute(ctx context.Context, action Action) (err error) {
	factoryDir := executor.FactoryDir
	configBranch := executor.ConfigBranch
	stateBranch := executor.StateBranch
	missingFactory := factoryDir == ""
	if missingFactory {
		factoryDir, err = os.Getwd()
		if err != nil {
			log.Error("current directory", "err", err)
			return
		}
	}
	log.Info("factory repo", "path", factoryDir, "config", configBranch, "state", stateBranch)

	sameBranch := configBranch == stateBranch
	if sameBranch {
		log.Warn("config branch and state branch are the same", "branch", configBranch)
		err = fmt.Errorf("config branch and state branch are the same")
		return
	}

	folders, err := listRepos(factoryDir)
	if err != nil {
		log.Error("list repos", "path", factoryDir, "err", err)
		return
	}
	none := len(folders) == 0
	if none {
		return
	}

	configDir, configAdded, err := ensureCheckout(ctx, factoryDir, configBranch)
	if err != nil {
		return
	}
	if configAdded {
		defer removeWorktree(ctx, factoryDir, configDir)
	}

	stateDir, stateAdded, err := ensureCheckout(ctx, factoryDir, stateBranch)
	if err != nil {
		return
	}
	if stateAdded {
		defer removeWorktree(ctx, factoryDir, stateDir)
	}

	for _, folder := range folders {
		err = apply(ctx, configDir, stateDir, action, folder)
		if err != nil {
			log.Error("apply", "owner", folder.owner, "name", folder.name, "err", err)
			return
		}
	}
	return
}

func listRepos(factoryDir string) (folders []repoFolder, err error) {
	reposDir := filepath.Join(factoryDir, "repos")
	owners, err := os.ReadDir(reposDir)
	missing := os.IsNotExist(err)
	if missing {
		err = nil
		return
	}
	if err != nil {
		return
	}

	for _, owner := range owners {
		ownerName := owner.Name()
		ownerRel := filepath.ToSlash(filepath.Join("repos", ownerName))
		ownerIsDir := owner.IsDir()
		if !ownerIsDir {
			err = fmt.Errorf("path is not repos/<owner>/<name>: %s", ownerRel)
			return
		}
		ownerDir := filepath.Join(reposDir, ownerName)
		names, readErr := os.ReadDir(ownerDir)
		if readErr != nil {
			err = readErr
			return
		}
		for _, name := range names {
			nameText := name.Name()
			rel := filepath.ToSlash(filepath.Join("repos", ownerName, nameText))
			nameIsDir := name.IsDir()
			if !nameIsDir {
				err = fmt.Errorf("path is not repos/<owner>/<name>: %s", rel)
				return
			}
			folder := repoFolder{
				owner: ownerName,
				name:  nameText,
				rel:   rel,
			}
			folders = append(folders, folder)
		}
	}

	sort.Slice(folders, func(i int, j int) bool {
		return folders[i].rel < folders[j].rel
	})
	return
}

func apply(ctx context.Context, configDir string, stateDir string, action Action, folder repoFolder) (err error) {
	configPath := filepath.Join(configDir, folder.rel, "config.db")
	_, err = os.Stat(configPath)
	missingConfig := os.IsNotExist(err)
	if missingConfig {
		log.Warn("config database is missing", "owner", folder.owner, "name", folder.name, "path", configPath)
		err = fmt.Errorf("config database is missing")
		return
	}
	if err != nil {
		log.Error("stat config", "path", configPath, "err", err)
		return
	}

	configDB, err := configdb.OpenIfMigrated(ctx, configPath)
	if err != nil {
		log.Error("open config", "path", configPath, "err", err)
		return
	}
	defer configDB.Close()

	statePath := filepath.Join(stateDir, folder.rel, "state.db")
	stateDB, err := statedb.Open(ctx, statePath)
	if err != nil {
		log.Error("open state", "path", statePath, "err", err)
		return
	}
	defer stateDB.Close()

	repo := Repo{
		Owner:  folder.owner,
		Name:   folder.name,
		Config: configDB,
		State:  stateDB,
	}
	id, err := newTickID()
	if err != nil {
		return
	}
	err = stateDB.Queries.CreateTick(ctx, id)
	if err != nil {
		log.Error("insert tick", "owner", folder.owner, "name", folder.name, "err", err)
		return
	}
	tick := stateentities.Tick{ID: id}
	message, err := action.Apply(ctx, repo, tick)
	if err != nil {
		return
	}

	err = stateDB.Close()
	if err != nil {
		log.Error("close state", "path", statePath, "err", err)
		return
	}
	storedID := tick.ID
	name := action.Name()
	repoName := folder.owner + "/" + folder.name
	subject := commitSubject(storedID, name, repoName, message)
	err = commitState(ctx, stateDir, folder.rel, subject)
	return
}

func newTickID() (id string, err error) {
	now := time.Now()
	timestamp := ulid.Timestamp(now)
	value, err := ulid.New(timestamp, rand.Reader)
	if err != nil {
		log.Error("new tick id", "err", err)
		return
	}
	id = value.String()
	return
}

func commitSubject(id string, name string, repoName string, message string) (subject string) {
	subject = fmt.Sprintf("[%s] [%s] [%s] %s", id, name, repoName, message)
	subject = strings.TrimSpace(subject)
	return
}

func ensureCheckout(ctx context.Context, factoryDir string, branch string) (dir string, added bool, err error) {
	list, err := git(ctx, factoryDir, "worktree", "list", "--porcelain")
	if err != nil {
		log.Error("list worktrees", "err", err)
		return
	}
	dir, found := findCheckout(list, branch)
	if found {
		log.Info("using existing checkout", "branch", branch, "path", dir)
		return
	}

	dir = filepath.Join(factoryDir, ".worktrees", branch)
	_, err = git(ctx, factoryDir, "worktree", "add", dir, branch)
	if err != nil {
		log.Error("add worktree", "branch", branch, "path", dir, "err", err)
		return
	}
	added = true
	log.Info("added worktree", "branch", branch, "path", dir)
	return
}

func removeWorktree(ctx context.Context, factoryDir string, worktreePath string) {
	_, err := git(ctx, factoryDir, "worktree", "remove", "--force", worktreePath)
	if err != nil {
		log.Warn("remove worktree", "path", worktreePath, "err", err)
	}
}

func commitState(ctx context.Context, stateDir string, rel string, message string) (err error) {
	statePath := filepath.Join(rel, "state.db")
	stateFile := filepath.ToSlash(statePath)
	_, err = git(ctx, stateDir, "add", "--", stateFile)
	if err != nil {
		log.Error("stage state", "dir", stateDir, "err", err)
		return
	}

	_, err = git(ctx, stateDir, "commit", "--allow-empty", "-m", message, "--", stateFile)
	if err != nil {
		log.Error("commit state", "dir", stateDir, "err", err)
		return
	}
	log.Info("committed state", "dir", stateDir)
	return
}
