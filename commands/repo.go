package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/statedb"
)

// Repo is one app repo with config and state open.
// Owner and Name are the repos/<owner>/<name> folder.
type Repo struct {
	Owner  string
	Name   string
	Config *configdb.DB
	State  *statedb.DB
}

type repoFolder struct {
	owner string
	name  string
	rel   string
}

type session struct {
	configDir string
	stateDir  string
	folders   []repoFolder
	cleanup   func()
}

// StateCommit is one git commit on the state branch.
// An empty Subject together with an error skips the commit.
type StateCommit struct {
	Subject string
	Body    string
}

// VisitRepos opens every app repo and calls visit.
// Config is read from configBranch and is not committed. State is opened from stateBranch.
// visit returns the state commit. An error with an empty subject does not commit.
// An error with a subject is committed, then returned.
// The first error stops the walk. Repos already committed stay committed.
func VisitRepos(ctx context.Context, factoryDir string, configBranch string, stateBranch string, visit func(context.Context, Repo) (StateCommit, error)) (err error) {
	opened, err := openSession(ctx, factoryDir, configBranch, stateBranch)
	if err != nil {
		return
	}
	defer opened.cleanup()

	for _, folder := range opened.folders {
		err = visitRepo(ctx, opened.configDir, opened.stateDir, folder, visit)
		if err != nil {
			log.Error("visit repo", "owner", folder.owner, "name", folder.name, "err", err)
			return
		}
	}
	return
}

func visitRepo(ctx context.Context, configDir string, stateDir string, folder repoFolder, visit func(context.Context, Repo) (StateCommit, error)) (err error) {
	repo, err := openRepo(ctx, configDir, stateDir, folder)
	if err != nil {
		return
	}
	defer repo.Config.Close()
	defer repo.State.Close()

	commit, err := visit(ctx, repo)
	visitErr := err
	failed := visitErr != nil
	emptySubject := commit.Subject == ""
	skipCommit := failed && emptySubject
	if skipCommit {
		return
	}

	err = repo.State.Close()
	if err != nil {
		log.Error("close state", "path", repo.State.FileFullPath, "err", err)
		return
	}
	err = commitState(ctx, stateDir, folder.rel, commit)
	if err != nil {
		return
	}
	err = visitErr
	return
}

func openSession(ctx context.Context, factoryDir string, configBranch string, stateBranch string) (opened session, err error) {
	opened.cleanup = func() {}

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
	stateDir, stateAdded, err := ensureCheckout(ctx, factoryDir, stateBranch)
	if err != nil {
		if configAdded {
			removeWorktree(ctx, factoryDir, configDir)
		}
		return
	}

	opened.configDir = configDir
	opened.stateDir = stateDir
	opened.folders = folders
	opened.cleanup = func() {
		if stateAdded {
			removeWorktree(ctx, factoryDir, stateDir)
		}
		if configAdded {
			removeWorktree(ctx, factoryDir, configDir)
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

func openRepo(ctx context.Context, configDir string, stateDir string, folder repoFolder) (repo Repo, err error) {
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

	statePath := filepath.Join(stateDir, folder.rel, "state.db")
	stateDB, err := statedb.Open(ctx, statePath)
	if err != nil {
		log.Error("open state", "path", statePath, "err", err)
		closeErr := configDB.Close()
		if closeErr != nil {
			log.Error("close config", "path", configPath, "err", closeErr)
		}
		return
	}

	repo = Repo{
		Owner:  folder.owner,
		Name:   folder.name,
		Config: configDB,
		State:  stateDB,
	}
	return
}

// CommitSubject is the state commit subject [id] [name] [repo] message.
func CommitSubject(id string, name string, repoName string, message string) (subject string) {
	subject = fmt.Sprintf("[%s] [%s] [%s] %s", id, name, repoName, message)
	subject = strings.TrimSpace(subject)
	return
}

func ensureCheckout(ctx context.Context, factoryDir string, branch string) (dir string, added bool, err error) {
	list, err := Git(ctx, factoryDir, "worktree", "list", "--porcelain")
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
	_, err = Git(ctx, factoryDir, "worktree", "add", dir, branch)
	if err != nil {
		log.Error("add worktree", "branch", branch, "path", dir, "err", err)
		return
	}
	added = true
	log.Info("added worktree", "branch", branch, "path", dir)
	return
}

func removeWorktree(ctx context.Context, factoryDir string, worktreePath string) {
	_, err := Git(ctx, factoryDir, "worktree", "remove", "--force", worktreePath)
	if err != nil {
		log.Warn("remove worktree", "path", worktreePath, "err", err)
	}
}

func commitState(ctx context.Context, stateDir string, rel string, commit StateCommit) (err error) {
	statePath := filepath.Join(rel, "state.db")
	stateFile := filepath.ToSlash(statePath)
	_, err = Git(ctx, stateDir, "add", "--", stateFile)
	if err != nil {
		log.Error("stage state", "dir", stateDir, "err", err)
		return
	}

	args := []string{"commit", "--allow-empty", "-m", commit.Subject}
	hasBody := commit.Body != ""
	if hasBody {
		args = append(args, "-m", commit.Body)
	}
	args = append(args, "--", stateFile)
	_, err = Git(ctx, stateDir, args...)
	if err != nil {
		log.Error("commit state", "dir", stateDir, "err", err)
		return
	}
	log.Info("committed state", "dir", stateDir)
	return
}
