package configdb

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/TigranOhanyan/gh-gcf/configdb/entities"
	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// DB is one SQLite file with this package's migrations applied.
type DB struct {
	DB           *sql.DB
	Queries      *entities.Queries
	FileFullPath string
	isClosed     bool
}

type migrationStrategy int

const (
	migrate migrationStrategy = iota
	fail
)

// OpenAndMigrate opens the SQLite file at path, turns foreign keys on, and applies this package's migrations.
// An existing file is migrated in place. A missing file is created.
func OpenAndMigrate(ctx context.Context, path string) (db *DB, err error) {
	db, err = open(ctx, path, migrate)
	return
}

// OpenIfMigrated opens the SQLite file at path when every migration in this binary is already applied.
// A file that is behind is left unchanged, and a missing file is not created.
func OpenIfMigrated(ctx context.Context, path string) (db *DB, err error) {
	db, err = open(ctx, path, fail)
	return
}

func open(ctx context.Context, path string, strategy migrationStrategy) (db *DB, err error) {
	missing := path == ""
	if missing {
		err = fmt.Errorf("database path cannot be empty")
		return
	}

	mustExist := strategy == fail
	if mustExist {
		_, err = os.Stat(path)
		if err != nil {
			return
		}
	}

	sqlDB, err := driver.Open(path, applyPragmas)
	if err != nil {
		return
	}
	sqlDB.SetMaxOpenConns(1)

	migrationsDir, err := fs.Sub(embedMigrations, "migrations")
	if err != nil {
		sqlDB.Close()
		return
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, migrationsDir)
	if err != nil {
		sqlDB.Close()
		return
	}

	err = applyStrategy(ctx, sqlDB, provider, strategy)
	if err != nil {
		sqlDB.Close()
		return
	}

	queries := entities.New(sqlDB)
	db = &DB{
		DB:           sqlDB,
		Queries:      queries,
		FileFullPath: path,
	}
	return
}

func applyStrategy(ctx context.Context, sqlDB *sql.DB, provider *goose.Provider, strategy migrationStrategy) (err error) {
	switch strategy {
	case migrate:
		_, err = provider.Up(ctx)
	case fail:
		err = requireApplied(ctx, sqlDB, provider)
	default:
		err = fmt.Errorf("unknown migration strategy")
	}
	return
}

func requireApplied(ctx context.Context, sqlDB *sql.DB, provider *goose.Provider) (err error) {
	queries := entities.New(sqlDB)
	versions, err := queries.GetAppliedMigrationVersions(ctx)
	if err != nil {
		err = fmt.Errorf("database migrations are not applied: %w", err)
		return
	}

	applied := map[int64]struct{}{}
	for _, version := range versions {
		applied[version] = struct{}{}
	}

	sources := provider.ListSources()
	for _, source := range sources {
		_, found := applied[source.Version]
		missing := !found
		if missing {
			err = fmt.Errorf("database migration %d is not applied", source.Version)
			return
		}
	}
	return
}

func applyPragmas(conn *sqlite3.Conn) (err error) {
	err = conn.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return
	}
	err = conn.Exec("PRAGMA journal_mode = DELETE")
	return
}

func (db *DB) Close() (err error) {
	if db.isClosed {
		return
	}
	db.isClosed = true
	err = db.DB.Close()
	return
}
