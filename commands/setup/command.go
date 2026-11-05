package setup

import (
	"context"
	"errors"
	"os"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func currentFactory(ctx context.Context) (dir string, err error) {
	dir, err = os.Getwd()
	if err != nil {
		log.Error("current directory", "err", err)
		return
	}
	err = commands.RequireCheckout(ctx, dir)
	if err != nil {
		log.Error("setup", "path", dir, "err", err)
	}
	return
}

func openRepoDB(ctx context.Context, factoryDir string, rawOwner string, rawRepo string) (db *configdb.DB, owner string, repo string, err error) {
	owner, err = normalizeSegment(rawOwner)
	if err != nil {
		return
	}
	repo, err = normalizeSegment(rawRepo)
	if err != nil {
		return
	}

	path := configPath(factoryDir, owner, repo)
	var missing bool
	db, missing, err = openConfig(ctx, path)
	if err != nil {
		return
	}
	if missing {
		err = errors.New(missingConfig)
		log.Info("config missing", "path", path)
	}
	return
}

func closeConfig(db *configdb.DB) {
	err := db.Close()
	if err != nil {
		log.Error("close config", "path", db.FileFullPath, "err", err)
	}
}

func withRepo(ctx context.Context, rawOwner string, rawRepo string, use func(dir string, owner string, repo string, db *configdb.DB) error) (err error) {
	dir, err := currentFactory(ctx)
	if err != nil {
		return
	}
	db, owner, repo, err := openRepoDB(ctx, dir, rawOwner, rawRepo)
	if err != nil {
		return
	}
	defer closeConfig(db)
	err = use(dir, owner, repo, db)
	return
}

func requireFlag(cmd *cobra.Command, name string) {
	err := cmd.MarkFlagRequired(name)
	if err != nil {
		log.Error("required flag", "name", name, "err", err)
	}
}
