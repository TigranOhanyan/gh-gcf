package development_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func Test_NewSetup_should_list_development(t *testing.T) {
	text := setupHelp(t, "development --help")
	assert.Contains(t, text, "Store a development")
	assert.Contains(t, text, "Print developments")
	assert.Contains(t, text, "Add or list check names")
}

func Test_SetupDevelopment_should_store_a_development(t *testing.T) {
	repoDir := readyDevelopmentRepo(t)

	line := `setup development add --owner octo --repo app --name " features " --starting " ready " --attention " stop " --standards-app " 42 " --specification-app " 43 "`
	_, err := runSetup(t, line)
	require.NoError(t, err)

	db := openStored(t, repoDir)
	var name, starting, attention string
	var standardsApp, specificationApp int64
	query := `SELECT name, starting_label, attention_label, standards_reviewer_app, specification_reviewer_app FROM development`
	err = db.DB.QueryRowContext(t.Context(), query).Scan(&name, &starting, &attention, &standardsApp, &specificationApp)
	require.NoError(t, err)
	assert.Equal(t, "features", name)
	assert.Equal(t, "ready", starting)
	assert.Equal(t, "stop", attention)
	assert.Equal(t, int64(42), standardsApp)
	assert.Equal(t, int64(43), specificationApp)
}

func Test_SetupDevelopment_should_record_a_development_without_writing_a_member(t *testing.T) {
	repoDir := readyDevelopmentRepo(t)

	line := "setup development add --owner octo --repo app --name features --starting ready --attention stop --standards-app 42 --specification-app 43"
	_, err := runSetup(t, line)
	require.NoError(t, err)

	db := openStored(t, repoDir)
	assert.Equal(t, 1, countRows(t, db, "development"))
	assertAbsentTable(t, db.DB, "member")
}

func Test_SetupDevelopment_should_list_nothing_before_a_development_is_stored(t *testing.T) {
	readyDevelopmentRepo(t)

	stdout, err := runSetup(t, "setup development list --owner octo --repo app")
	require.NoError(t, err)
	assert.Empty(t, stdout)
}

func Test_SetupDevelopment_should_list_developments(t *testing.T) {
	readyDevelopmentRepo(t)
	addStoredLabel(t, "todo", "cccccc")
	addStoredDevelopment(t, "features", "ready", "stop", "42", "43")
	addStoredDevelopment(t, "bugs", "todo", "stop", "99", "100")

	stdout, err := runSetup(t, "setup development list --owner octo --repo app")
	require.NoError(t, err)
	assert.Equal(t, "bugs  todo  stop  99  100\nfeatures  ready  stop  42  43\n", stdout)
}

func Test_SetupDevelopment_should_list_check_names(t *testing.T) {
	readyDevelopmentRepo(t)
	addStoredDevelopment(t, "features", "ready", "stop", "42", "43")
	addStoredCheck(t, "features", "lint")
	addStoredCheck(t, "features", "ci / unit")

	stdout, err := runSetup(t, "setup development check list --owner octo --repo app --development features")
	require.NoError(t, err)
	assert.Equal(t, "ci / unit\nlint\n", stdout)
}

func Test_SetupDevelopment_should_store_a_check_name(t *testing.T) {
	repoDir := readyDevelopmentRepo(t)
	addStoredDevelopment(t, "features", "ready", "stop", "42", "43")

	line := `setup development check add --owner octo --repo app --development " features " --name " ci / unit "`
	_, err := runSetup(t, line)
	require.NoError(t, err)

	db := openStored(t, repoDir)
	var developmentName, checkName string
	query := `SELECT development_name, name FROM development_check`
	err = db.DB.QueryRowContext(t.Context(), query).Scan(&developmentName, &checkName)
	require.NoError(t, err)
	assert.Equal(t, "features", developmentName)
	assert.Equal(t, "ci / unit", checkName)
}

func Test_SetupDevelopment_should_refuse_the_same_reviewer_app(t *testing.T) {
	repoDir := readyDevelopmentRepo(t)

	line := `setup development add --owner octo --repo app --name features --starting ready --attention stop --standards-app 42 --specification-app 42`
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must differ")

	db := openStored(t, repoDir)
	assert.Equal(t, 0, countRows(t, db, "development"))
	assert.Equal(t, 0, countRows(t, db, "development_check"))
}

func Test_SetupDevelopment_should_refuse_a_repeated_starting_label(t *testing.T) {
	repoDir := readyDevelopmentRepo(t)
	addStoredDevelopment(t, "features", "ready", "stop", "42", "43")

	line := `setup development add --owner octo --repo app --name bugs --starting ready --attention stop --standards-app 99 --specification-app 100`
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already used")

	db := openStored(t, repoDir)
	names := developmentNames(t, db)
	require.Len(t, names, 1)
	assert.Equal(t, "features", names[0])
	assert.Equal(t, 0, countRows(t, db, "development_check"))
}

func Test_SetupDevelopment_should_refuse_a_reviewer_app_another_development_uses(t *testing.T) {
	repoDir := readyDevelopmentRepo(t)
	addStoredDevelopment(t, "features", "ready", "stop", "42", "43")
	addStoredLabel(t, "todo", "cccccc")

	line := `setup development add --owner octo --repo app --name bugs --starting todo --attention stop --standards-app 43 --specification-app 99`
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already used")

	db := openStored(t, repoDir)
	names := developmentNames(t, db)
	require.Len(t, names, 1)
	assert.Equal(t, "features", names[0])
	assert.Equal(t, 0, countRows(t, db, "development_check"))
}

func readyDevelopmentRepo(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = readyRepo(t)
	addStoredApp(t, "42")
	attachStoredApp(t, "42")
	addStoredApp(t, "43")
	attachStoredApp(t, "43")
	addStoredApp(t, "99")
	attachStoredApp(t, "99")
	addStoredApp(t, "100")
	attachStoredApp(t, "100")
	return
}

func addStoredCheck(t *testing.T, developmentName string, name string) {
	t.Helper()
	line := fmt.Sprintf("setup development check add --owner octo --repo app --development %s --name %q", developmentName, name)
	_, err := runSetup(t, line)
	require.NoError(t, err)
}

func addStoredDevelopment(t *testing.T, name string, starting string, attention string, standardsApp string, specificationApp string) {
	t.Helper()
	line := fmt.Sprintf("setup development add --owner octo --repo app --name %s --starting %s --attention %s --standards-app %s --specification-app %s", name, starting, attention, standardsApp, specificationApp)
	_, err := runSetup(t, line)
	require.NoError(t, err)
}

func developmentNames(t *testing.T, db *configdb.DB) (names []string) {
	t.Helper()
	rows, err := db.DB.QueryContext(t.Context(), `SELECT name FROM development ORDER BY name`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		require.NoError(t, err)
		names = append(names, name)
	}
	err = rows.Err()
	require.NoError(t, err)
	return
}

func assertAbsentTable(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var found string
	query := `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`
	err := db.QueryRowContext(t.Context(), query, name).Scan(&found)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func countRows(t *testing.T, db *configdb.DB, table string) (count int) {
	t.Helper()
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	err := db.DB.QueryRowContext(t.Context(), query).Scan(&count)
	require.NoError(t, err)
	return
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
