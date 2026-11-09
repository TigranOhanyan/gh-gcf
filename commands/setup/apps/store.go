package apps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands/setup/common"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func CreateFile(factoryDir string, raw string) (id int64, err error) {
	id, err = common.ParseAppID(raw)
	if err != nil {
		return
	}

	dir := filepath.Join(factoryDir, "apps")
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.Error("create app", "path", dir, "err", err)
		return
	}

	canonical := strconv.FormatInt(id, 10)
	path := filepath.Join(dir, canonical)
	_, statErr := os.Stat(path)
	exists := statErr == nil
	if exists {
		err = fmt.Errorf("app %s already exists", canonical)
		log.Info("app exists", "app", canonical)
		return
	}
	missing := os.IsNotExist(statErr)
	if statErr != nil && !missing {
		err = statErr
		log.Error("stat app", "path", path, "err", err)
		return
	}

	empty := []byte{}
	err = os.WriteFile(path, empty, 0644)
	if err != nil {
		log.Error("create app", "path", path, "err", err)
		return
	}
	log.Info("create app", "app", id, "path", path)
	return
}

func Attach(ctx context.Context, db *configdb.DB, id int64) (err error) {
	ids, err := db.Queries.ListGithubApps(ctx)
	if err != nil {
		log.Error("list apps", "err", err)
		return
	}
	for _, existing := range ids {
		same := existing == id
		if same {
			err = fmt.Errorf("app %d already exists", id)
			log.Info("app exists", "app", id)
			return
		}
	}

	err = db.Queries.CreateGithubApp(ctx, id)
	if err != nil {
		log.Error("attach app", "app", id, "err", err)
		return
	}
	log.Info("attach app", "app", id)
	return
}
