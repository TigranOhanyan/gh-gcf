package bind

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	workflowfile "github.com/TigranOhanyan/gh-gcf/github"
)

const (
	workflowRel   = ".github/workflows/factory-tick.yml"
	gitignoreRel  = ".worktrees/.gitignore"
	hintRel       = ".worktrees/internal_use_only"
	commitMessage = "bind factory"
	versionToken  = "__GCF_VERSION__"
	devVersion    = "dev"
)

// Version is the release tag of this binary. The release build sets it with -X.
// A local build leaves it as dev, and bind refuses to run.
var Version = devVersion

func NewBind() (cmd *cobra.Command) {
	var factoryRepo string

	cmd = &cobra.Command{
		Use:   "bind",
		Short: "Install the tick workflow and migrate config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Bind(cmd.Context(), factoryRepo)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&factoryRepo, "factory-repo", "", "path to the factory checkout")
	return
}

func Bind(ctx context.Context, factoryDir string) (err error) {
	err = requireRelease()
	if err != nil {
		log.Error("gh-gcf version", "version", Version, "err", err)
		return
	}

	missingFactory := factoryDir == ""
	if missingFactory {
		factoryDir, err = os.Getwd()
		if err != nil {
			log.Error("current directory", "err", err)
			return
		}
	}

	err = requireMain(ctx, factoryDir)
	if err != nil {
		log.Error("factory repo", "path", factoryDir, "err", err)
		return
	}
	log.Info("factory repo", "path", factoryDir)

	mainDir, added, err := ensureMain(ctx, factoryDir)
	if err != nil {
		return
	}
	if added {
		defer removeWorktree(ctx, factoryDir, mainDir)
	}

	err = installWorkflow(mainDir)
	if err != nil {
		log.Error("install workflow", "path", mainDir, "err", err)
		return
	}

	err = installWorktrees(mainDir)
	if err != nil {
		log.Error("install worktrees", "path", mainDir, "err", err)
		return
	}

	configRels, err := migrateConfigs(ctx, mainDir)
	if err != nil {
		log.Error("migrate config", "path", mainDir, "err", err)
		return
	}

	files := []string{workflowRel, gitignoreRel, hintRel}
	files = append(files, configRels...)
	err = commitMain(ctx, mainDir, files)
	return
}

func requireMain(ctx context.Context, factoryDir string) (err error) {
	top, err := git(ctx, factoryDir, "rev-parse", "--show-toplevel")
	if err != nil {
		err = fmt.Errorf("factory repo is not a git checkout")
		return
	}
	root := strings.TrimSpace(string(top))
	factoryAbs, err := filepath.Abs(factoryDir)
	if err != nil {
		return
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return
	}
	sameRoot := filepath.Clean(factoryAbs) == filepath.Clean(rootAbs)
	if !sameRoot {
		err = fmt.Errorf("factory repo is not a git checkout")
		return
	}

	_, err = git(ctx, factoryDir, "rev-parse", "--verify", "refs/heads/main")
	if err != nil {
		err = fmt.Errorf("factory repo has no main branch")
		return
	}
	return
}

func installWorkflow(mainDir string) (err error) {
	workflowPath := filepath.Join(mainDir, filepath.FromSlash(workflowRel))
	err = os.Remove(workflowPath)
	missing := os.IsNotExist(err)
	if missing {
		err = nil
	}
	if err != nil {
		return
	}

	workflowDir := filepath.Dir(workflowPath)
	err = os.MkdirAll(workflowDir, 0755)
	if err != nil {
		return
	}
	contents, err := workflowContents()
	if err != nil {
		return
	}
	err = os.WriteFile(workflowPath, contents, 0644)
	return
}

func requireRelease() (err error) {
	missing := Version == "" || Version == devVersion
	if missing {
		err = fmt.Errorf("gh-gcf version is not a release")
	}
	return
}

func workflowContents() (contents []byte, err error) {
	token := []byte(versionToken)
	hasToken := bytes.Contains(workflowfile.FactoryTick, token)
	if !hasToken {
		err = fmt.Errorf("workflow template has no version token")
		return
	}
	version := []byte(Version)
	contents = bytes.ReplaceAll(workflowfile.FactoryTick, token, version)
	return
}

func installWorktrees(mainDir string) (err error) {
	worktreesDir := filepath.Join(mainDir, ".worktrees")
	err = os.MkdirAll(worktreesDir, 0755)
	if err != nil {
		return
	}

	gitignorePath := filepath.Join(mainDir, filepath.FromSlash(gitignoreRel))
	err = os.WriteFile(gitignorePath, worktreesGitignore, 0644)
	if err != nil {
		return
	}

	hintPath := filepath.Join(mainDir, filepath.FromSlash(hintRel))
	empty := []byte{}
	err = os.WriteFile(hintPath, empty, 0644)
	return
}

func migrateConfigs(ctx context.Context, mainDir string) (rels []string, err error) {
	apps, err := listApps(mainDir)
	if err != nil {
		return
	}

	for _, app := range apps {
		configRel := filepath.ToSlash(filepath.Join(app, "config.db"))
		configPath := filepath.Join(mainDir, filepath.FromSlash(configRel))
		_, statErr := os.Stat(configPath)
		missing := os.IsNotExist(statErr)
		if missing {
			continue
		}
		if statErr != nil {
			err = statErr
			return
		}

		db, openErr := configdb.OpenAndMigrate(ctx, configPath)
		if openErr != nil {
			err = openErr
			return
		}
		err = db.Close()
		if err != nil {
			return
		}
		rels = append(rels, configRel)
		log.Info("migrated config", "path", configRel)
	}
	return
}

func commitMain(ctx context.Context, mainDir string, files []string) (err error) {
	for _, rel := range files {
		_, err = git(ctx, mainDir, "add", "--", rel)
		if err != nil {
			log.Error("stage bind file", "path", rel, "err", err)
			return
		}
	}

	args := []string{"commit", "-m", commitMessage, "--"}
	args = append(args, files...)
	output, err := git(ctx, mainDir, args...)
	if err == nil {
		log.Info("committed bind", "dir", mainDir)
		return
	}
	marker := []byte("nothing to commit")
	unchanged := bytes.Contains(output, marker)
	if unchanged {
		log.Info("bind files are unchanged", "dir", mainDir)
		err = nil
		return
	}
	log.Error("commit bind", "dir", mainDir, "err", err)
	return
}

func listApps(mainDir string) (rels []string, err error) {
	reposDir := filepath.Join(mainDir, "repos")
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
		ownerIsDir := owner.IsDir()
		if !ownerIsDir {
			err = fmt.Errorf("path is not repos/<owner>/<name>: repos/%s", ownerName)
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
			rels = append(rels, rel)
		}
	}

	sort.Strings(rels)
	return
}

func ensureMain(ctx context.Context, factoryDir string) (dir string, added bool, err error) {
	list, err := git(ctx, factoryDir, "worktree", "list", "--porcelain")
	if err != nil {
		log.Error("list worktrees", "err", err)
		return
	}
	dir, found := findCheckout(list, "main")
	if found {
		log.Info("using existing checkout", "branch", "main", "path", dir)
		return
	}

	dir = filepath.Join(factoryDir, ".worktrees", "main")
	_, err = git(ctx, factoryDir, "worktree", "add", dir, "main")
	if err != nil {
		log.Error("add worktree", "branch", "main", "path", dir, "err", err)
		return
	}
	added = true
	log.Info("added worktree", "branch", "main", "path", dir)
	return
}

func removeWorktree(ctx context.Context, factoryDir string, worktreePath string) {
	_, err := git(ctx, factoryDir, "worktree", "remove", "--force", worktreePath)
	if err != nil {
		log.Warn("remove worktree", "path", worktreePath, "err", err)
	}
}
