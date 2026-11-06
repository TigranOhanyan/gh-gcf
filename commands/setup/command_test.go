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

func Test_NewSetup_should_list_member(t *testing.T) {
	text := setupHelp(t, "member --help")
	assert.Contains(t, text, "Add or list members")
	assert.Contains(t, text, "Store a builder and its instruction file")
	assert.Contains(t, text, "Store a reviewer and its instruction file")
	assert.Contains(t, text, "Store a fixer and its instruction file")
	assert.Contains(t, text, "Print stored members")
}

func Test_NewSetup_should_list_check(t *testing.T) {
	text := setupHelp(t, "check --help")
	assert.Contains(t, text, "Add or list checks")
	assert.Contains(t, text, "Store a check")
	assert.Contains(t, text, "Print checks")
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

func Test_SetupMember_should_store_a_builder(t *testing.T) {
	repoDir := readyRepo(t)

	line := `setup member add-builder --owner octo --repo app --name " features " --starting " ready " --attention " stop "`
	_, err := runSetup(t, line)
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

	info, err := os.Stat(instructionPath(repoDir, "features"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func Test_SetupMember_should_refuse_a_repeated_starting_label(t *testing.T) {
	repoDir := readyRepo(t)
	addStoredBuilder(t)

	line := "setup member add-builder --owner octo --repo app --name bugs --starting ready --attention stop"
	_, err := runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already used")

	db := openStored(t, repoDir)
	rows, err := db.Queries.ListBuilderMembers(t.Context())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "features", rows[0].Name)
	assert.Equal(t, "ready", rows[0].StartingLabel)
	assert.Equal(t, "stop", rows[0].AttentionLabel)

	_, err = os.Stat(instructionPath(repoDir, "bugs"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
	info, err := os.Stat(instructionPath(repoDir, "features"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func Test_SetupMember_should_store_a_reviewer(t *testing.T) {
	repoDir := readyRepo(t)
	addStoredApp(t, "42")
	attachStoredApp(t, "42")
	addStoredBuilder(t)

	line := "setup member add-reviewer --owner octo --repo app --builder features --name standards --app 42 --attention ready"
	_, err := runSetup(t, line)
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

func Test_SetupMember_should_store_a_fixer(t *testing.T) {
	repoDir := readyRepo(t)
	addStoredBuilder(t)

	line := "setup member add-fixer --owner octo --repo app --builder features --name patches --attention ready"
	_, err := runSetup(t, line)
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

	info, err := os.Stat(instructionPath(repoDir, "patches"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())
}

func Test_SetupMember_should_list_members(t *testing.T) {
	enterCheckout(t)
	addRepo(t)
	addStoredLabel(t, "ready", "ededed")
	addStoredLabel(t, "stop", "ff0000")
	addStoredApp(t, "42")
	attachStoredApp(t, "42")
	addStoredBuilder(t)
	_, err := runSetup(t, "setup member add-reviewer --owner octo --repo app --builder features --name standards --app 42 --attention ready")
	require.NoError(t, err)
	_, err = runSetup(t, "setup member add-fixer --owner octo --repo app --builder features --name patches --attention stop")
	require.NoError(t, err)

	stdout, err := runSetup(t, "setup member list --owner octo --repo app")
	require.NoError(t, err)
	expected := "builder  features  ready  stop\nreviewer  standards  features  42  ready\nfixer  patches  features  stop\n"
	assert.Equal(t, expected, stdout)
}

func Test_SetupCheck_should_store_a_check(t *testing.T) {
	repoDir := readyRepo(t)
	addStoredBuilder(t)

	line := `setup check add --owner octo --repo app --builder features --name " ci / unit "`
	_, err := runSetup(t, line)
	require.NoError(t, err)

	db := openStored(t, repoDir)
	names, err := db.Queries.ListGithubChecks(t.Context(), "features")
	require.NoError(t, err)
	require.Len(t, names, 1)
	assert.Equal(t, "ci / unit", names[0])
}

func Test_SetupCheck_should_refuse_a_duplicate(t *testing.T) {
	repoDir := readyRepo(t)
	addStoredBuilder(t)
	line := `setup check add --owner octo --repo app --builder features --name "ci / unit"`
	_, err := runSetup(t, line)
	require.NoError(t, err)

	_, err = runSetup(t, line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	db := openStored(t, repoDir)
	names, err := db.Queries.ListGithubChecks(t.Context(), "features")
	require.NoError(t, err)
	require.Len(t, names, 1)
	assert.Equal(t, "ci / unit", names[0])
}

func Test_SetupCheck_should_list_checks(t *testing.T) {
	enterCheckout(t)
	addRepo(t)
	addStoredLabel(t, "ready", "ededed")
	addStoredLabel(t, "stop", "ff0000")
	addStoredBuilder(t)
	_, err := runSetup(t, `setup check add --owner octo --repo app --builder features --name "ci / unit"`)
	require.NoError(t, err)

	stdout, err := runSetup(t, "setup check list --owner octo --repo app --builder features")
	require.NoError(t, err)
	assert.Equal(t, "ci / unit\n", stdout)
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

func addStoredBuilder(t *testing.T) {
	t.Helper()
	line := "setup member add-builder --owner octo --repo app --name features --starting ready --attention stop"
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
