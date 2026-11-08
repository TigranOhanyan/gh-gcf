package setup_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func Test_NewSetup_should_list_repo(t *testing.T) {
	text := setupHelp(t, "repo --help")
	assert.Contains(t, text, "Add or list app repos")
	assert.Contains(t, text, "Store an app repo")
	assert.Contains(t, text, "Print app repos")
}

func Test_NewSetup_should_list_label(t *testing.T) {
	text := setupHelp(t, "label --help")
	assert.Contains(t, text, "Add or list labels")
	assert.Contains(t, text, "Store a label")
	assert.Contains(t, text, "Print stored labels")
}

func Test_NewSetup_should_list_apps(t *testing.T) {
	text := setupHelp(t, "apps --help")
	assert.Contains(t, text, "Add or list Apps")
	assert.Contains(t, text, "Register an App")
	assert.Contains(t, text, "Print Apps")
	assert.Contains(t, text, "Attach an App to an app repo")
}

func Test_SetupRepo_should_store_an_empty_config_database(t *testing.T) {
	repoDir := enterCheckout(t)

	_, err := runSetup(t, `setup repo add --owner " octo " --name " app "`)
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_SetupRepo_should_refuse_a_duplicate(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)

	_, err := runSetup(t, "setup repo add --owner octo --name app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_SetupRepo_should_list_repos(t *testing.T) {
	enterCheckout(t)
	_, err := runSetup(t, "setup repo add --owner octo --name app")
	require.NoError(t, err)
	_, err = runSetup(t, "setup repo add --owner octo --name lib")
	require.NoError(t, err)

	stdout, err := runSetup(t, "setup repo list")
	require.NoError(t, err)
	assert.Equal(t, "octo/app\nocto/lib\n", stdout)
}

func Test_SetupLabel_should_store_a_label(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)

	startOfTest := time.Now().Add(-time.Second)
	line := `setup label add --owner octo --repo app --name " bug " --color "#EDEDED"`
	_, err := runSetup(t, line)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	labels := storedLabels(t, repoDir, "octo", "app")
	require.Len(t, labels, 1)
	assert.Equal(t, "bug", labels[0].Name)
	assert.Equal(t, "ededed", labels[0].Color)
	registered, err := time.Parse(time.RFC3339Nano, labels[0].RegisteredAt)
	require.NoError(t, err)
	assert.WithinRange(t, registered, startOfTest, startOfCheck)
}

func Test_SetupLabel_should_refuse_a_duplicate(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)
	addStoredLabel(t, "bug", "ededed")

	line := "setup label add --owner octo --repo app --name bug --color ff0000"
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	labels := storedLabels(t, repoDir, "octo", "app")
	require.Len(t, labels, 1)
	assert.Equal(t, "bug", labels[0].Name)
	assert.Equal(t, "ededed", labels[0].Color)
}

func Test_SetupLabel_should_refuse_a_missing_argument(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)

	line := "setup label add --owner octo --repo app --color ededed"
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_SetupLabel_should_list_labels(t *testing.T) {
	enterCheckout(t)
	addRepo(t)
	addStoredLabel(t, "ready", "aabbcc")
	addStoredLabel(t, "bug", "ededed")

	stdout, err := runSetup(t, "setup label list --owner octo --repo app")
	require.NoError(t, err)
	assert.Equal(t, "bug  ededed\nready  aabbcc\n", stdout)
}

func Test_SetupLabel_should_leave_the_commit_to_the_user(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)
	addStoredLabel(t, "bug", "ededed")

	message := mustGit(t, repoDir, "log -1 --format=%s")
	assert.Equal(t, "init", strings.TrimSpace(string(message)))
	tracked := mustGit(t, repoDir, "ls-files repos/octo/app/config.db")
	assert.Empty(t, strings.TrimSpace(string(tracked)))
}

func Test_SetupLabel_should_refuse_a_directory_that_is_not_a_git_checkout(t *testing.T) {
	repoDir := filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)
	t.Chdir(repoDir)

	line := "setup label add --owner octo --repo app --name bug --color ededed"
	_, err = runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a git checkout")

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_SetupApps_should_store_an_app_file(t *testing.T) {
	repoDir := enterCheckout(t)

	_, err := runSetup(t, "setup apps add --id 42")
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(repoDir, "apps", "42"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func Test_SetupApps_should_refuse_a_duplicate(t *testing.T) {
	repoDir := enterCheckout(t)
	addStoredApp(t, "42")

	_, err := runSetup(t, "setup apps add --id 42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	info, err := os.Stat(filepath.Join(repoDir, "apps", "42"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func Test_SetupApps_should_refuse_an_id_that_is_not_a_positive_integer(t *testing.T) {
	repoDir := enterCheckout(t)

	_, err := runSetup(t, "setup apps add --id nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive integer")

	_, err = os.Stat(filepath.Join(repoDir, "apps"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_SetupApps_should_list_apps(t *testing.T) {
	enterCheckout(t)
	addStoredApp(t, "42")
	addStoredApp(t, "7")

	stdout, err := runSetup(t, "setup apps list")
	require.NoError(t, err)
	assert.Equal(t, "7\n42\n", stdout)
}

func Test_SetupApps_should_attach_an_app(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)
	addStoredApp(t, "42")

	_, err := runSetup(t, "setup apps attach --owner octo --repo app --id 42")
	require.NoError(t, err)

	db := openStored(t, repoDir)
	ids, err := db.Queries.ListGithubApps(t.Context())
	require.NoError(t, err)
	require.Len(t, ids, 1)
	assert.Equal(t, int64(42), ids[0])
}

func Test_SetupApps_should_list_apps_attached_to_a_repo(t *testing.T) {
	enterCheckout(t)
	addRepo(t)
	addStoredApp(t, "42")
	attachStoredApp(t, "42")

	stdout, err := runSetup(t, "setup apps list --owner octo --repo app")
	require.NoError(t, err)
	assert.Equal(t, "42\n", stdout)
}

func Test_SetupApps_should_refuse_an_unregistered_app(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)

	_, err := runSetup(t, "setup apps attach --owner octo --repo app --id 42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not registered")

	db := openStored(t, repoDir)
	ids, err := db.Queries.ListGithubApps(t.Context())
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func Test_SetupApps_should_refuse_a_repo_without_an_owner(t *testing.T) {
	repoDir := enterCheckout(t)
	addStoredApp(t, "42")

	_, err := runSetup(t, "setup apps list --repo app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "owner and repo must both be set")

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
	info, err := os.Stat(filepath.Join(repoDir, "apps", "42"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func setupHelp(t *testing.T, line string) (text string) {
	t.Helper()
	full := "setup " + line
	text, err := runSetup(t, full)
	require.NoError(t, err)
	return
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

func addStoredLabel(t *testing.T, name string, color string) {
	t.Helper()
	line := fmt.Sprintf("setup label add --owner octo --repo app --name %q --color %q", name, color)
	_, err := runSetup(t, line)
	require.NoError(t, err)
}

func addStoredApp(t *testing.T, id string) {
	t.Helper()
	line := fmt.Sprintf("setup apps add --id %q", id)
	_, err := runSetup(t, line)
	require.NoError(t, err)
}

func attachStoredApp(t *testing.T, id string) {
	t.Helper()
	line := fmt.Sprintf("setup apps attach --owner octo --repo app --id %q", id)
	_, err := runSetup(t, line)
	require.NoError(t, err)
}

func readyRepo(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = enterCheckout(t)
	addRepo(t)
	addStoredLabel(t, "ready", "ededed")
	addStoredLabel(t, "stop", "ff0000")
	return
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
