package configdb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TigranOhanyan/gh-gcf/configdb/entities"
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

func Test_Open_should_migrate_a_new_database(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_Open_should_migrate_a_new_database.db")
	removeDatabase(t, path)

	db, err := Open(ctx, path)
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

func Test_Open_should_leave_an_existing_database_unchanged(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_Open_should_leave_an_existing_database_unchanged.db")
	removeDatabase(t, path)

	db, err := Open(ctx, path)
	require.NoError(t, err)

	params := entities.CreateConfigParams{
		Name:  "starting-label",
		Value: "todo",
	}
	err = db.Queries.CreateConfig(ctx, params)
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)

	db, err = Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	row, err := db.Queries.GetConfig(ctx, "starting-label")
	require.NoError(t, err)
	assert.Equal(t, "starting-label", row.Name)
	assert.Equal(t, "todo", row.Value)
}

func Test_Open_should_refuse_an_empty_path(t *testing.T) {
	ctx := t.Context()

	db, err := Open(ctx, "")
	require.Error(t, err)
	assert.Nil(t, db)
}
