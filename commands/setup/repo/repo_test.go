package repo_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func Test_SetupRepo_should_refuse_a_directory_that_is_not_a_git_checkout(t *testing.T) {
	repoDir := filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)
	t.Chdir(repoDir)

	_, err = runSetup(t, "setup repo add --owner octo --name app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a git checkout")

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_SetupRepo_should_refuse_a_subdirectory_of_a_git_checkout(t *testing.T) {
	repoDir := setupCheckout(t)
	sub := filepath.Join(repoDir, "nested")
	err := os.MkdirAll(sub, 0755)
	require.NoError(t, err)
	t.Chdir(sub)

	_, err = runSetup(t, "setup repo add --owner octo --name app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a git checkout")

	_, err = os.Stat(filepath.Join(sub, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_SetupRepo_should_create_an_empty_config_database(t *testing.T) {
	repoDir := enterCheckout(t)

	_, err := runSetup(t, "setup repo add --owner octo --name app")
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_SetupRepo_should_list_nothing_before_a_repo_is_stored(t *testing.T) {
	repoDir := enterCheckout(t)

	stdout, err := runSetup(t, "setup repo list")
	require.NoError(t, err)
	assert.Empty(t, stdout)

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_SetupRepo_should_leave_the_commit_to_the_user(t *testing.T) {
	repoDir := enterCheckout(t)

	_, err := runSetup(t, "setup repo add --owner octo --name app")
	require.NoError(t, err)

	configPath := filepath.Join(repoDir, "repos", "octo", "app", "config.db")
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log -1 --format=%s")
	assert.Equal(t, "init", strings.TrimSpace(string(message)))

	tracked := mustGit(t, repoDir, "ls-files repos/octo/app/config.db")
	assert.Empty(t, strings.TrimSpace(string(tracked)))

	_, err = os.Stat(filepath.Join(repoDir, "repos", "octo", "app", "state.db"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_SetupRepo_should_write_config_on_the_checked_out_branch(t *testing.T) {
	repoDir := setupCheckout(t)
	_ = mustGit(t, repoDir, "checkout -b review")
	t.Chdir(repoDir)

	_, err := runSetup(t, "setup repo add --owner octo --name app")
	require.NoError(t, err)

	branch := mustGit(t, repoDir, "branch --show-current")
	assert.Equal(t, "review", strings.TrimSpace(string(branch)))

	_, err = os.Stat(filepath.Join(repoDir, "repos", "octo", "app", "config.db"))
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log -1 --format=%s")
	assert.Equal(t, "init", strings.TrimSpace(string(message)))
}

func Test_SetupRepo_should_refuse_an_existing_repo_folder(t *testing.T) {
	repoDir := enterCheckout(t)
	_, err := runSetup(t, "setup repo add --owner octo --name app")
	require.NoError(t, err)

	_, err = runSetup(t, "setup repo add --owner octo --name app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_SetupRepo_should_refuse_a_name_that_is_not_a_single_path_segment(t *testing.T) {
	repoDir := enterCheckout(t)

	_, err := runSetup(t, "setup repo add --owner octo/app --name web")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "single path segment")

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_SetupRepo_should_store_a_repo(t *testing.T) {
	repoDir := enterCheckout(t)

	_, err := runSetup(t, "setup repo add --owner octo --name app")
	require.NoError(t, err)

	stdout, err := runSetup(t, "setup repo list")
	require.NoError(t, err)
	assert.Equal(t, "octo/app\n", stdout)

	_, err = os.Stat(filepath.Join(repoDir, "repos", "octo", "app", "config.db"))
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(repoDir, "repos", "octoapp"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_SetupRepo_should_refuse_an_empty_repo_name(t *testing.T) {
	repoDir := enterCheckout(t)

	_, err := runSetup(t, `setup repo add --owner octo --name ""`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is empty")

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
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
