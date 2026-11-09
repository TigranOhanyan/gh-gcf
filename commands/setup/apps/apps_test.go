package apps_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func Test_SetupApps_should_register_an_app(t *testing.T) {
	repoDir := enterCheckout(t)

	_, err := runSetup(t, "setup apps add --id 42")
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(repoDir, "apps", "42"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func Test_SetupApps_should_attach_a_registered_app(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)
	_, err := runSetup(t, "setup apps add --id 42")
	require.NoError(t, err)

	_, err = runSetup(t, "setup apps attach --owner octo --repo app --id 42")
	require.NoError(t, err)

	db := openStored(t, repoDir)
	ids, err := db.Queries.ListGithubApps(t.Context())
	require.NoError(t, err)
	require.Len(t, ids, 1)
	assert.Equal(t, int64(42), ids[0])
}

func enterCheckout(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = setupCheckout(t)
	t.Chdir(repoDir)
	return
}

func runSetup(t *testing.T, line string) (stdout string, err error) {
	t.Helper()
	command := root.NewRoot()
	var out bytes.Buffer
	var errOut bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&errOut)
	wrapped := commands.NewCmd(command)
	err = wrapped.Shell(t.Context(), line)
	stdout = out.String()
	return
}

func addRepo(t *testing.T) {
	t.Helper()
	_, err := runSetup(t, "setup repo add --owner octo --name app")
	require.NoError(t, err)
}

func openStored(t *testing.T, repoDir string) (db *configdb.DB) {
	t.Helper()
	path := filepath.Join(repoDir, "repos", "octo", "app", "config.db")
	var err error
	db, err = configdb.OpenIfMigrated(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(func() {
		closeErr := db.Close()
		failed := closeErr != nil
		if failed {
			t.Errorf("close config: %v", closeErr)
		}
	})
	return
}

func setupCheckout(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	_ = mustGit(t, repoDir, "init -b main")
	_ = mustGit(t, repoDir, "config user.email test@example.com")
	_ = mustGit(t, repoDir, "config user.name test")
	_ = mustGit(t, repoDir, "commit --allow-empty -m init")
	return
}
