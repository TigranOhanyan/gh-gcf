package commands

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"charm.land/log/v2"
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
	configMissing := configPath == ""
	if configMissing {
		log.Warn("config file was not provided")
		err = fmt.Errorf("config file was not provided")
		return
	}
	log.Info("config file provided", "path", configPath)

	stateMissing := statePath == ""
	if stateMissing {
		log.Warn("state file was not provided")
		err = fmt.Errorf("state file was not provided")
		return
	}
	log.Info("state file provided", "path", statePath)

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
