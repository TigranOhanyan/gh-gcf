package configdb

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

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

// Open opens the SQLite file at path, turns foreign keys on, and applies this package's migrations.
// An existing file is migrated in place.
func Open(ctx context.Context, path string) (db *DB, err error) {
	missing := path == ""
	if missing {
		err = fmt.Errorf("database path cannot be empty")
		return
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

	_, err = provider.Up(ctx)
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
