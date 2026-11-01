package statedb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tickID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

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

	ids, err := db.Queries.GetTicks(ctx)
	require.NoError(t, err)
	assert.Empty(t, ids)

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

	err = db.Queries.CreateTick(ctx, tickID)
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)

	db, err = Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	ids, err := db.Queries.GetTicks(ctx)
	require.NoError(t, err)
	require.Len(t, ids, 1)
	assert.Equal(t, tickID, ids[0].ID)
}

func Test_CreateTick_should_refuse_an_id_that_is_not_26_characters(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_CreateTick_should_refuse_an_id_that_is_not_26_characters.db")
	removeDatabase(t, path)

	db, err := Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	shortID := "short"
	err = db.Queries.CreateTick(ctx, shortID)
	require.Error(t, err)

	ids, err := db.Queries.GetTicks(ctx)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func Test_Open_should_refuse_an_empty_path(t *testing.T) {
	ctx := t.Context()

	db, err := Open(ctx, "")
	require.Error(t, err)
	assert.Nil(t, db)
}
