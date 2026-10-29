package copy

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	stateentities "github.com/TigranOhanyan/gh-gcf/statedb/entities"
)

func NewCopy() (cmd *cobra.Command) {
	var factoryRepo string
	var configBranch string
	var stateBranch string

	cmd = &cobra.Command{
		Use:   "copy",
		Short: "Copy config values into state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Copy(cmd.Context(), factoryRepo, configBranch, stateBranch)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&factoryRepo, "factory-repo", "", "path to the factory checkout")
	flags.StringVar(&configBranch, "config-branch", "main", "branch that holds config.db")
	flags.StringVar(&stateBranch, "state-branch", "state", "branch that holds state.db")
	return
}

func Copy(ctx context.Context, factoryDir string, configBranch string, stateBranch string) (err error) {
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

	configPath := filepath.Join(configDir, "config.db")
	_, err = os.Stat(configPath)
	missingConfig := os.IsNotExist(err)
	if missingConfig {
		log.Warn("config database is missing", "path", configPath)
		err = fmt.Errorf("config database is missing")
		return
	}
	if err != nil {
		log.Error("stat config", "path", configPath, "err", err)
		return
	}

	err = copyRows(ctx, configPath, filepath.Join(stateDir, "state.db"))
	if err != nil {
		return
	}

	err = commitState(ctx, stateDir)
	return
}

func ensureCheckout(ctx context.Context, factoryDir string, branch string) (dir string, added bool, err error) {
	list, err := runGit(ctx, factoryDir, "worktree", "list", "--porcelain")
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
	_, err = runGit(ctx, factoryDir, "worktree", "add", dir, branch)
	if err != nil {
		log.Error("add worktree", "branch", branch, "path", dir, "err", err)
		return
	}
	added = true
	log.Info("added worktree", "branch", branch, "path", dir)
	return
}

func removeWorktree(ctx context.Context, factoryDir string, worktreePath string) {
	_, err := runGit(ctx, factoryDir, "worktree", "remove", "--force", worktreePath)
	if err != nil {
		log.Warn("remove worktree", "path", worktreePath, "err", err)
	}
}

func copyRows(ctx context.Context, configPath string, statePath string) (err error) {
	configDB, err := configdb.Open(ctx, configPath)
	if err != nil {
		log.Error("open config", "path", configPath, "err", err)
		return
	}
	defer configDB.Close()

	stateDB, err := statedb.Open(ctx, statePath)
	if err != nil {
		log.Error("open state", "path", statePath, "err", err)
		return
	}
	defer stateDB.Close()

	rows, err := configDB.Queries.GetConfigs(ctx)
	if err != nil {
		log.Error("read config", "path", configPath, "err", err)
		return
	}

	tx, err := stateDB.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin state transaction", "path", statePath, "err", err)
		return
	}
	defer rollback(tx, &err)

	queries := stateDB.Queries.WithTx(tx)
	for _, row := range rows {
		params := stateentities.CreateStateParams{
			Name:  row.Name,
			Value: row.Value,
		}
		err = queries.CreateState(ctx, params)
		if err != nil {
			log.Error("insert state row", "name", row.Name, "err", err)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Error("commit state", "path", statePath, "err", err)
		return
	}
	log.Info("copied config rows into state", "rows", len(rows))
	return
}

func commitState(ctx context.Context, stateDir string) (err error) {
	_, err = runGit(ctx, stateDir, "add", "--", "state.db")
	if err != nil {
		log.Error("stage state", "dir", stateDir, "err", err)
		return
	}

	output, err := runGit(ctx, stateDir, "commit", "-m", "copy config into state", "--", "state.db")
	if err == nil {
		log.Info("committed state", "dir", stateDir)
		return
	}
	unchanged := bytes.Contains(output, []byte("nothing to commit"))
	if unchanged {
		log.Info("state database is unchanged", "dir", stateDir)
		err = nil
		return
	}
	log.Error("commit state", "dir", stateDir, "err", err)
	return
}

func rollback(tx *sql.Tx, errp *error) {
	rollbackErr := tx.Rollback()
	if rollbackErr == nil {
		return
	}
	done := errors.Is(rollbackErr, sql.ErrTxDone)
	if done {
		return
	}
	if *errp == nil {
		*errp = rollbackErr
		log.Error("roll back state", "err", rollbackErr)
		return
	}
	log.Warn("roll back state", "err", rollbackErr)
}
