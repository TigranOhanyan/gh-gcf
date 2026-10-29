package configdb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TigranOhanyan/gh-gcf/configdb/entities"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func removeDatabase(t *testing.T, path string) {
	t.Helper()
	err := os.Remove(path)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		require.NoError(t, err)
	}
}

func Test_OpenAndMigrate_should_migrate_a_new_database(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_OpenAndMigrate_should_migrate_a_new_database.db")
	removeDatabase(t, path)

	db, err := OpenAndMigrate(ctx, path)
	require.NoError(t, err)
	require.NotNil(t, db.Queries)

	rows, err := db.Queries.GetConfigs(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows)

	err = db.Close()
	require.NoError(t, err)

	stat, err := os.Stat(path)
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0))
}

func Test_OpenAndMigrate_should_leave_an_existing_database_unchanged(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_OpenAndMigrate_should_leave_an_existing_database_unchanged.db")
	removeDatabase(t, path)

	db, err := OpenAndMigrate(ctx, path)
	require.NoError(t, err)

	params := entities.CreateConfigParams{
		Name:  "starting-label",
		Value: "todo",
	}
	err = db.Queries.CreateConfig(ctx, params)
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)

	db, err = OpenAndMigrate(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	row, err := db.Queries.GetConfig(ctx, "starting-label")
	require.NoError(t, err)
	assert.Equal(t, "starting-label", row.Name)
	assert.Equal(t, "todo", row.Value)
}

func Test_OpenAndMigrate_should_refuse_an_empty_path(t *testing.T) {
	ctx := t.Context()

	db, err := OpenAndMigrate(ctx, "")
	require.Error(t, err)
	assert.Nil(t, db)
}

func Test_OpenIfMigrated_should_open_a_database_at_the_current_version(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_OpenIfMigrated_should_open_a_database_at_the_current_version.db")
	removeDatabase(t, path)

	db, err := OpenAndMigrate(ctx, path)
	require.NoError(t, err)

	params := entities.CreateConfigParams{
		Name:  "starting-label",
		Value: "todo",
	}
	err = db.Queries.CreateConfig(ctx, params)
	require.NoError(t, err)
	err = db.Close()
	require.NoError(t, err)

	db, err = OpenIfMigrated(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	row, err := db.Queries.GetConfig(ctx, "starting-label")
	require.NoError(t, err)
	assert.Equal(t, "starting-label", row.Name)
	assert.Equal(t, "todo", row.Value)
}

func Test_OpenIfMigrated_should_refuse_a_database_that_is_behind(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_OpenIfMigrated_should_refuse_a_database_that_is_behind.db")
	removeDatabase(t, path)

	sqlDB, err := driver.Open(path)
	require.NoError(t, err)
	err = sqlDB.Close()
	require.NoError(t, err)

	db, err := OpenIfMigrated(ctx, path)
	require.Error(t, err)
	assert.Nil(t, db)

	sqlDB, err = driver.Open(path)
	require.NoError(t, err)
	defer sqlDB.Close()

	var name string
	err = sqlDB.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'config'`).Scan(&name)
	assert.Error(t, err)
	err = sqlDB.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'goose_db_version'`).Scan(&name)
	assert.Error(t, err)
}

func Test_OpenIfMigrated_should_refuse_a_missing_file(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_OpenIfMigrated_should_refuse_a_missing_file.db")
	removeDatabase(t, path)

	db, err := OpenIfMigrated(ctx, path)
	require.Error(t, err)
	assert.Nil(t, db)

	_, err = os.Stat(path)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_OpenIfMigrated_should_refuse_an_empty_path(t *testing.T) {
	ctx := t.Context()

	db, err := OpenIfMigrated(ctx, "")
	require.Error(t, err)
	assert.Nil(t, db)
}
