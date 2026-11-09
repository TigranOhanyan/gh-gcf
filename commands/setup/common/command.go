package common

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

const MissingConfig = "config.db is missing"
const maxLabelName = 50

func CurrentFactory(ctx context.Context) (dir string, err error) {
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

func OpenConfig(ctx context.Context, path string) (db *configdb.DB, missing bool, err error) {
	_, err = os.Stat(path)
	missing = os.IsNotExist(err)
	if missing {
		err = nil
		return
	}
	if err != nil {
		log.Error("stat config", "path", path, "err", err)
		return
	}

	db, err = configdb.OpenAndMigrate(ctx, path)
	if err != nil {
		log.Error("open config", "path", path, "err", err)
	}
	return
}

func CloseConfig(db *configdb.DB) {
	err := db.Close()
	if err != nil {
		log.Error("close config", "path", db.FileFullPath, "err", err)
	}
}

func ConfigPath(factoryDir string, owner string, name string) (path string) {
	path = filepath.Join(factoryDir, "repos", owner, name, "config.db")
	return
}

func WithRepo(ctx context.Context, rawOwner string, rawRepo string, use func(dir string, owner string, repo string, db *configdb.DB) error) (err error) {
	dir, err := CurrentFactory(ctx)
	if err != nil {
		return
	}
	db, owner, repo, err := openRepoDB(ctx, dir, rawOwner, rawRepo)
	if err != nil {
		return
	}
	defer CloseConfig(db)
	err = use(dir, owner, repo, db)
	return
}

func RequireFlag(cmd *cobra.Command, name string) {
	err := cmd.MarkFlagRequired(name)
	if err != nil {
		log.Error("required flag", "name", name, "err", err)
	}
}

func NormalizeSegment(raw string) (segment string, err error) {
	segment = strings.TrimSpace(raw)
	empty := segment == ""
	if empty {
		err = fmt.Errorf("name is empty")
		segment = ""
		return
	}

	hasSlash := strings.Contains(segment, "/")
	hasBackslash := strings.Contains(segment, "\\")
	hasSep := hasSlash || hasBackslash
	dot := segment == "." || segment == ".."
	bad := hasSep || dot
	if bad {
		err = fmt.Errorf("name must be a single path segment")
		segment = ""
	}
	return
}

func NormalizeLabelName(raw string) (name string, err error) {
	name = strings.TrimSpace(raw)
	empty := name == ""
	if empty {
		err = fmt.Errorf("label name is empty")
		return
	}

	count := utf8.RuneCountInString(name)
	tooLong := count > maxLabelName
	if tooLong {
		err = fmt.Errorf("label name is longer than 50 characters")
	}
	return
}

func ParseAppID(raw string) (id int64, err error) {
	text := strings.TrimSpace(raw)
	parsed, parseErr := strconv.ParseInt(text, 10, 64)
	canonical := strconv.FormatInt(parsed, 10)
	bad := parseErr != nil || parsed <= 0 || canonical != text
	if bad {
		err = fmt.Errorf("app id must be a positive integer")
		return
	}
	id = parsed
	return
}

func NormalizeCheckName(raw string) (name string, err error) {
	name = strings.TrimSpace(raw)
	empty := name == ""
	if empty {
		err = fmt.Errorf("check name is empty")
	}
	return
}

func openRepoDB(ctx context.Context, factoryDir string, rawOwner string, rawRepo string) (db *configdb.DB, owner string, repo string, err error) {
	owner, err = NormalizeSegment(rawOwner)
	if err != nil {
		return
	}
	repo, err = NormalizeSegment(rawRepo)
	if err != nil {
		return
	}

	path := ConfigPath(factoryDir, owner, repo)
	var missing bool
	db, missing, err = OpenConfig(ctx, path)
	if err != nil {
		return
	}
	if missing {
		err = errors.New(MissingConfig)
		log.Info("config missing", "path", path)
	}
	return
}
