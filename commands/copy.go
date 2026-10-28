package commands

import (
	"context"
	"database/sql"
	"errors"

	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	stateentities "github.com/TigranOhanyan/gh-gcf/statedb/entities"
)

func NewCopy() (cmd *cobra.Command) {
	var configPath string
	var statePath string

	cmd = &cobra.Command{
		Use:   "copy",
		Short: "Copy config values into state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Copy(cmd.Context(), configPath, statePath)
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&configPath, "config", "", "path to config.db")
	flags.StringVar(&statePath, "state", "", "path to state.db")

	err := cmd.MarkFlagRequired("config")
	if err != nil {
		panic(err)
	}
	err = cmd.MarkFlagRequired("state")
	if err != nil {
		panic(err)
	}
	return
}

func Copy(ctx context.Context, configPath string, statePath string) (err error) {
	configDB, err := configdb.Open(ctx, configPath)
	if err != nil {
		return
	}
	defer configDB.Close()

	stateDB, err := statedb.Open(ctx, statePath)
	if err != nil {
		return
	}
	defer stateDB.Close()

	rows, err := configDB.Queries.GetConfigs(ctx)
	if err != nil {
		return
	}

	tx, err := stateDB.DB.BeginTx(ctx, nil)
	if err != nil {
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
			return
		}
	}

	err = tx.Commit()
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
	}
}
