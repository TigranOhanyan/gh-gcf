package label_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

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

func Test_SetupLabel_should_refuse_a_duplicate_label(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)
	addStoredLabel(t, "bug", "ededed")

	line := "setup label add --owner octo --repo app --name bug --color ffffff"
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	labels := storedLabels(t, repoDir, "octo", "app")
	require.Len(t, labels, 1)
	assert.Equal(t, "bug", labels[0].Name)
	assert.Equal(t, "ededed", labels[0].Color)
}

func Test_SetupLabel_should_refuse_a_color_that_is_not_hex(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)

	line := "setup label add --owner octo --repo app --name bug --color red"
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "six hex digits")

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_SetupLabel_should_refuse_a_label_name_longer_than_50_characters(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)

	longName := strings.Repeat("a", 51)
	line := fmt.Sprintf("setup label add --owner octo --repo app --name %q --color %q", longName, "ededed")
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "longer than 50 characters")

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_SetupLabel_should_store_a_label_name_of_50_characters(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)

	name := strings.Repeat("a", 50)
	line := fmt.Sprintf("setup label add --owner octo --repo app --name %q --color %q", name, "ededed")
	_, err := runSetup(t, line)
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	require.Len(t, labels, 1)
	assert.Equal(t, name, labels[0].Name)
}

func Test_SetupLabel_should_leave_a_missing_config_database_uncreated(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)
	path := filepath.Join(repoDir, "repos", "octo", "app", "config.db")
	err := os.Remove(path)
	require.NoError(t, err)

	line := "setup label add --owner octo --repo app --name bug --color ededed"
	_, err = runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config.db is missing")

	_, err = os.Stat(path)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_SetupLabel_should_migrate_a_config_database_that_is_behind(t *testing.T) {
	repoDir := enterCheckout(t)
	dir := filepath.Join(repoDir, "repos", "octo", "app")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	writeEmptyDatabase(t, filepath.Join(dir, "config.db"))

	line := "setup label add --owner octo --repo app --name bug --color ededed"
	_, err = runSetup(t, line)
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	require.Len(t, labels, 1)
	assert.Equal(t, "bug", labels[0].Name)
}

func Test_SetupLabel_should_leave_the_database_unchanged_when_the_color_is_missing(t *testing.T) {
	repoDir := enterCheckout(t)
	addRepo(t)

	line := "setup label add --owner octo --repo app --name bug"
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "color")

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
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
