package setup

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func Test_Setup_should_store_a_builder_member(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "ready", "ededed", "ready  ededed")
	submitLabel(t, session, "stop", "ff0000", "stop  ff0000")
	openBuilders(t, session)
	session.write("a")
	session.wait("builders>add")
	session.write(" features ")
	session.wait("features")
	session.write("\r")
	session.wait("add>starting")
	session.write("\r")
	session.wait("add>attention")
	session.write("\r")
	session.wait("features  ready  stop")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	rows, err := db.Queries.ListBuilderMembers(t.Context())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "features", rows[0].Name)
	assert.Equal(t, "ready", rows[0].StartingLabel)
	assert.Equal(t, "stop", rows[0].AttentionLabel)

	var memberName, role, attention string
	err = db.DB.QueryRowContext(t.Context(), `SELECT name, role, attention_label FROM member`).Scan(&memberName, &role, &attention)
	require.NoError(t, err)
	assert.Equal(t, "features", memberName)
	assert.Equal(t, "builder", role)
	assert.Equal(t, "stop", attention)

	var builderName, builderRole, starting string
	err = db.DB.QueryRowContext(t.Context(), `SELECT name, role, starting_label FROM builder_member`).Scan(&builderName, &builderRole, &starting)
	require.NoError(t, err)
	assert.Equal(t, "features", builderName)
	assert.Equal(t, "builder", builderRole)
	assert.Equal(t, "ready", starting)

	assert.Equal(t, []string{"name", "role", "attention_label"}, columnNames(t, db, "member"))

	info, err := os.Stat(instructionPath(repoDir, "features"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func Test_Setup_should_leave_config_unchanged_when_a_builder_has_one_label(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "ready", "ededed", "ready  ededed")
	openBuilders(t, session)
	session.write("a")
	session.wait("builders>add")
	session.write("features")
	session.wait("features")
	session.write("\r")
	session.wait("add>starting")
	session.write("\r")
	session.wait("No other label.")
	session.escape()
	session.wait("add>starting")
	session.escape()
	session.wait("No builders yet.")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	names, err := db.Queries.ListMemberNames(t.Context())
	require.NoError(t, err)
	assert.Empty(t, names)
	_, err = os.Stat(instructionPath(repoDir, "features"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_hide_a_starting_label_that_is_already_used(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "ready", "ededed", "ready  ededed")
	submitLabel(t, session, "stop", "ff0000", "stop  ff0000")
	openBuilders(t, session)
	addBuilder(t, session, "features")
	session.write("a")
	session.wait("builders>add")
	session.write("bugs")
	session.wait("bugs")
	session.write("\r")
	session.wait("add>starting")
	screen := session.screen()
	assert.NotContains(t, screen, "│ ready")
	assert.Contains(t, screen, "│ stop")
	session.escape()
	session.wait("features  ready  stop")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	rows, err := db.Queries.ListBuilderMembers(t.Context())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "features", rows[0].Name)
}

func Test_Setup_should_store_a_reviewer_member(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	registerAndAttach(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "ready", "ededed", "ready  ededed")
	submitLabel(t, session, "stop", "ff0000", "stop  ff0000")
	openBuilders(t, session)
	addBuilder(t, session, "features")
	session.write("\r")
	session.wait("reviewers")
	session.write("\r")
	session.wait("No reviewers yet.")
	session.write("a")
	session.wait("reviewers>add")
	session.write("standards")
	session.wait("standards")
	session.write("\r")
	session.wait("add>app")
	session.write("\r")
	session.wait("add>attention")
	session.write("\r")
	session.wait("standards  42")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	var name, role, attention, builder string
	var appID int64
	query := `SELECT m.name, m.role, m.attention_label, r.builder_name, r.github_app
		FROM reviewer_member r JOIN member m ON m.name = r.name`
	err = db.DB.QueryRowContext(t.Context(), query).Scan(&name, &role, &attention, &builder, &appID)
	require.NoError(t, err)
	assert.Equal(t, "standards", name)
	assert.Equal(t, "reviewer", role)
	assert.Equal(t, "ready", attention)
	assert.Equal(t, "features", builder)
	assert.Equal(t, int64(42), appID)

	info, err := os.Stat(instructionPath(repoDir, "standards"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func Test_Setup_should_hide_an_app_a_reviewer_already_uses(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	registerAndAttach(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "ready", "ededed", "ready  ededed")
	submitLabel(t, session, "stop", "ff0000", "stop  ff0000")
	openBuilders(t, session)
	addBuilder(t, session, "features")
	session.write("\r")
	session.wait("reviewers")
	session.write("\r")
	session.wait("No reviewers yet.")
	session.write("a")
	session.wait("reviewers>add")
	session.write("standards")
	session.wait("standards")
	session.write("\r")
	session.wait("add>app")
	session.write("\r")
	session.wait("add>attention")
	session.write("\r")
	session.wait("standards  42")
	session.write("a")
	session.wait("reviewers>add")
	session.write("spec")
	session.wait("spec")
	session.write("\r")
	session.wait("add>app")
	screen := session.screen()
	assert.NotContains(t, screen, "│ 42")
	session.escape()
	session.wait("standards  42")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	rows, err := db.Queries.ListReviewerMembers(t.Context(), "features")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "standards", rows[0].Name)
	_, err = os.Stat(instructionPath(repoDir, "spec"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_refuse_a_member_name_that_already_exists(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	registerAndAttach(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "ready", "ededed", "ready  ededed")
	submitLabel(t, session, "stop", "ff0000", "stop  ff0000")
	openBuilders(t, session)
	addBuilder(t, session, "features")
	session.write("\r")
	session.wait("reviewers")
	session.write("\r")
	session.wait("No reviewers yet.")
	session.write("a")
	session.wait("reviewers>add")
	session.write("features")
	session.wait("features")
	session.write("\r")
	session.wait("already exists")
	session.escape()
	session.wait("No reviewers yet.")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	rows, err := db.Queries.ListReviewerMembers(t.Context(), "features")
	require.NoError(t, err)
	assert.Empty(t, rows)
	_, err = os.Stat(instructionPath(repoDir, "features"))
	require.NoError(t, err)
}

func Test_Setup_should_store_one_fixer_for_a_builder(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "ready", "ededed", "ready  ededed")
	submitLabel(t, session, "stop", "ff0000", "stop  ff0000")
	openBuilders(t, session)
	addBuilder(t, session, "features")
	session.write("\r")
	session.wait("reviewers")
	session.write("j")
	session.write("\r")
	session.wait("No fixer yet.")
	session.write("a")
	session.wait("fixer>add")
	session.write("patches")
	session.wait("patches")
	session.write("\r")
	session.wait("add>attention")
	session.write("\r")
	session.wait("patches  ready")
	session.write("a")
	session.wait("fixer already exists")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	var name, role, attention, builder string
	query := `SELECT m.name, m.role, m.attention_label, f.builder_name
		FROM fixer_member f JOIN member m ON m.name = f.name`
	err = db.DB.QueryRowContext(t.Context(), query).Scan(&name, &role, &attention, &builder)
	require.NoError(t, err)
	assert.Equal(t, "patches", name)
	assert.Equal(t, "fixer", role)
	assert.Equal(t, "ready", attention)
	assert.Equal(t, "features", builder)

	var count int
	err = db.DB.QueryRowContext(t.Context(), `SELECT count(*) FROM fixer_member`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	info, err := os.Stat(instructionPath(repoDir, "patches"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func Test_Setup_should_store_a_check_for_a_builder(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "ready", "ededed", "ready  ededed")
	submitLabel(t, session, "stop", "ff0000", "stop  ff0000")
	openBuilders(t, session)
	addBuilder(t, session, "features")
	openChecks(t, session)
	session.write("a")
	session.wait("checks>add")
	session.write(" ci / unit ")
	session.wait("ci / unit")
	session.write("\r")
	session.wait("a add")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	names, err := db.Queries.ListGithubChecks(t.Context(), "features")
	require.NoError(t, err)
	require.Len(t, names, 1)
	assert.Equal(t, "ci / unit", names[0])
}

func Test_Setup_should_refuse_a_duplicate_check_for_the_same_builder(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "ready", "ededed", "ready  ededed")
	submitLabel(t, session, "stop", "ff0000", "stop  ff0000")
	openBuilders(t, session)
	addBuilder(t, session, "features")
	openChecks(t, session)
	session.write("a")
	session.wait("checks>add")
	session.write("ci / unit")
	session.write("\r")
	session.wait("ci / unit")
	session.write("a")
	session.wait("checks>add")
	session.write("ci / unit")
	session.write("\r")
	session.wait("already exists")
	session.escape()
	session.wait("a add")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	names, err := db.Queries.ListGithubChecks(t.Context(), "features")
	require.NoError(t, err)
	require.Len(t, names, 1)
}

func Test_Setup_should_store_the_same_check_name_on_another_builder(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	session := selectRepo(t, repoDir)
	submitLabel(t, session, "bug", "ededed", "bug  ededed")
	submitLabel(t, session, "hold", "ff0000", "hold  ff0000")
	submitLabel(t, session, "ready", "00ff00", "ready  00ff00")
	submitLabel(t, session, "stop", "0000ff", "stop  0000ff")
	openBuilders(t, session)
	addReadyBuilder(t, session, "features")
	openChecks(t, session)
	session.write("a")
	session.wait("checks>add")
	session.write("ci / unit")
	session.write("\r")
	session.wait("ci / unit")
	session.escape()
	session.wait("reviewers")
	session.escape()
	session.wait("features  ready  stop")
	addSecondBuilder(t, session)
	openChecks(t, session)
	session.write("a")
	session.wait("checks>add")
	session.write("ci / unit")
	session.write("\r")
	session.wait("ci / unit")
	err := session.quit()
	require.NoError(t, err)

	db := openStored(t, repoDir)
	var count int
	err = db.DB.QueryRowContext(t.Context(), `SELECT count(*) FROM github_check WHERE name = ?`, "ci / unit").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func openBuilders(t *testing.T, session *terminalSession) {
	t.Helper()
	session.escape()
	session.wait("builders")
	session.write("j")
	session.write("j")
	session.write("\r")
	session.wait("No builders yet.")
}

func addBuilder(t *testing.T, session *terminalSession, name string) {
	t.Helper()
	session.write("a")
	session.wait("builders>add")
	session.write(name)
	session.wait(name)
	session.write("\r")
	session.wait("add>starting")
	session.write("\r")
	session.wait("add>attention")
	session.write("\r")
	session.wait(name + "  ready  stop")
}

func addReadyBuilder(t *testing.T, session *terminalSession, name string) {
	t.Helper()
	session.write("a")
	session.wait("builders>add")
	session.write(name)
	session.wait(name)
	session.write("\r")
	session.wait("add>starting")
	session.write("j")
	session.write("j")
	session.write("\r")
	session.wait("add>attention")
	session.write("j")
	session.write("j")
	session.write("\r")
	session.wait(name + "  ready  stop")
}

func addSecondBuilder(t *testing.T, session *terminalSession) {
	t.Helper()
	session.write("a")
	session.wait("builders>add")
	session.write("bugs")
	session.wait("bugs")
	session.write("\r")
	session.wait("add>starting")
	session.write("\r")
	session.wait("add>attention")
	session.write("\r")
	session.wait("bugs  bug  hold")
}

func openChecks(t *testing.T, session *terminalSession) {
	t.Helper()
	session.write("\r")
	session.wait("reviewers")
	session.write("j")
	session.write("j")
	session.write("\r")
	session.wait(">checks")
}

func registerAndAttach(t *testing.T, repoDir string) {
	t.Helper()
	session := startSetup(t, repoDir)
	session.wait("repos")
	session.write("j")
	session.write("\r")
	session.wait("No apps yet.")
	session.write("a")
	session.wait("app id:")
	session.write("42")
	session.write("\r")
	session.wait("GITHUB_APP_42")
	session.escape()
	session.wait("repos")
	session.write("\r")
	session.wait("octo/app")
	session.write("\r")
	session.wait("labels")
	session.write("j")
	session.write("\r")
	session.wait("No apps yet.")
	session.write("a")
	session.wait("42")
	session.write("\r")
	session.wait("GITHUB_APP_42")
	err := session.quit()
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

func instructionPath(repoDir string, name string) (path string) {
	path = filepath.Join(repoDir, "repos", "octo", "app", "members", name+".md")
	return
}

func columnNames(t *testing.T, db *configdb.DB, table string) (names []string) {
	t.Helper()
	rows, err := db.DB.QueryContext(t.Context(), `PRAGMA table_info(`+table+`)`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		err = rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		require.NoError(t, err)
		names = append(names, name)
	}
	err = rows.Err()
	require.NoError(t, err)
	return
}
