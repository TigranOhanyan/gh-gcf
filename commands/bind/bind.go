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
	cmd = &cobra.Command{
		Use:   "bind",
		Short: "Install the tick workflow and migrate config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			dir, err := os.Getwd()
			if err != nil {
				log.Error("current directory", "err", err)
				return
			}
			err = Bind(cmd.Context(), dir)
			return
		},
	}
	return
}

func Bind(ctx context.Context, factoryDir string) (err error) {
	err = requireRelease()
	if err != nil {
		log.Error("gh-gcf version", "version", Version, "err", err)
		return
	}

	err = requireCheckout(ctx, factoryDir)
	if err != nil {
		log.Error("factory repo", "path", factoryDir, "err", err)
		return
	}
	log.Info("factory repo", "path", factoryDir)

	err = installWorkflow(factoryDir)
	if err != nil {
		log.Error("install workflow", "path", factoryDir, "err", err)
		return
	}

	err = installWorktrees(factoryDir)
	if err != nil {
		log.Error("install worktrees", "path", factoryDir, "err", err)
		return
	}

	configRels, err := migrateConfigs(ctx, factoryDir)
	if err != nil {
		log.Error("migrate config", "path", factoryDir, "err", err)
		return
	}

	files := []string{workflowRel, gitignoreRel, hintRel}
	files = append(files, configRels...)
	err = commitCheckout(ctx, factoryDir, files)
	return
}

func requireCheckout(ctx context.Context, factoryDir string) (err error) {
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
	return
}

func installWorkflow(factoryDir string) (err error) {
	workflowPath := filepath.Join(factoryDir, filepath.FromSlash(workflowRel))
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

func installWorktrees(factoryDir string) (err error) {
	worktreesDir := filepath.Join(factoryDir, ".worktrees")
	err = os.MkdirAll(worktreesDir, 0755)
	if err != nil {
		return
	}

	gitignorePath := filepath.Join(factoryDir, filepath.FromSlash(gitignoreRel))
	err = os.WriteFile(gitignorePath, worktreesGitignore, 0644)
	if err != nil {
		return
	}

	hintPath := filepath.Join(factoryDir, filepath.FromSlash(hintRel))
	empty := []byte{}
	err = os.WriteFile(hintPath, empty, 0644)
	return
}

func migrateConfigs(ctx context.Context, factoryDir string) (rels []string, err error) {
	apps, err := listApps(factoryDir)
	if err != nil {
		return
	}

	for _, app := range apps {
		configRel := filepath.ToSlash(filepath.Join(app, "config.db"))
		configPath := filepath.Join(factoryDir, filepath.FromSlash(configRel))
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

func commitCheckout(ctx context.Context, factoryDir string, files []string) (err error) {
	for _, rel := range files {
		_, err = git(ctx, factoryDir, "add", "--", rel)
		if err != nil {
			log.Error("stage bind file", "path", rel, "err", err)
			return
		}
	}

	args := []string{"commit", "-m", commitMessage, "--"}
	args = append(args, files...)
	output, err := git(ctx, factoryDir, args...)
	if err == nil {
		log.Info("committed bind", "dir", factoryDir)
		return
	}
	marker := []byte("nothing to commit")
	unchanged := bytes.Contains(output, marker)
	if unchanged {
		log.Info("bind files are unchanged", "dir", factoryDir)
		err = nil
		return
	}
	log.Error("commit bind", "dir", factoryDir, "err", err)
	return
}

func listApps(factoryDir string) (rels []string, err error) {
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
