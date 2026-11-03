package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func Test_Setup_should_refuse_a_directory_that_is_not_a_git_checkout(t *testing.T) {
	repoDir := filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)
	t.Chdir(repoDir)

	cmd := NewSetup()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err = cmd.ExecuteContext(t.Context())
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_refuse_a_subdirectory_of_a_git_checkout(t *testing.T) {
	repoDir := setupCheckout(t)
	sub := filepath.Join(repoDir, "nested")
	err := os.MkdirAll(sub, 0755)
	require.NoError(t, err)
	t.Chdir(sub)

	cmd := NewSetup()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err = cmd.ExecuteContext(t.Context())
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(sub, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func setupCheckout(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	_ = mustGit(t, repoDir, "init", "-b", "main")
	_ = mustGit(t, repoDir, "config", "user.email", "test@example.com")
	_ = mustGit(t, repoDir, "config", "user.name", "test")
	_ = mustGit(t, repoDir, "commit", "--allow-empty", "-m", "init")
	return
}

func storedLabels(t *testing.T, repoDir string, owner string, name string) (labels []configrecords.Label) {
	t.Helper()
	path := filepath.Join(repoDir, "repos", owner, name, "config.db")
	db, err := configdb.OpenIfMigrated(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(func() {
		closeErr := db.Close()
		failed := closeErr != nil
		if failed {
			t.Errorf("close config: %v", closeErr)
		}
	})
	labels, err = db.Queries.ListLabels(t.Context())
	require.NoError(t, err)
	return
}

func writeEmptyDatabase(t *testing.T, path string) {
	t.Helper()
	sqlDB, err := driver.Open(path)
	require.NoError(t, err)
	err = sqlDB.Ping()
	require.NoError(t, err)
	err = sqlDB.Close()
	require.NoError(t, err)
}
