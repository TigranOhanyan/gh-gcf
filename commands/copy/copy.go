package copy

import (
	"context"
	"database/sql"
	"errors"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/repos"
	stateentities "github.com/TigranOhanyan/gh-gcf/statedb/entities"
)

const commitMessage = "copy config into state"

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
	action := copyAction{}
	err = repos.Run(ctx, factoryDir, configBranch, stateBranch, commitMessage, action)
	return
}

type copyAction struct{}

func (copyAction) Apply(ctx context.Context, repo repos.Repo) (err error) {
	rows, err := repo.Config.Queries.GetConfigs(ctx)
	if err != nil {
		log.Error("read config", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}

	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin state transaction", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}
	defer rollback(tx, &err)

	queries := repo.State.Queries.WithTx(tx)
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
		log.Error("commit state", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}
	log.Info("copied config rows into state", "owner", repo.Owner, "name", repo.Name, "rows", len(rows))
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
