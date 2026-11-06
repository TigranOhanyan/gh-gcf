package tick_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

func Test_Tick_should_print_the_stored_tick_id(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)

	_, err := ulid.Parse(id)
	assert.NoError(t, err)
	rows := ticksAtTip(t, repoDir, "repos/octo/app")
	require.Len(t, rows, 1)
	assert.Equal(t, id, rows[0].ID)
}

func Test_Tick_should_commit_the_tick_subject_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)

	message := mustGit(t, repoDir, "log -1 --format=%s state")
	expected := fmt.Sprintf("[%s] [tick] [octo/app] created", id)
	assert.Equal(t, expected, strings.TrimSpace(string(message)))
}

func Test_Tick_should_store_one_tick_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)
	rel := "repos/octo/app"
	id := runTick(t, repoDir)

	_, err := ulid.Parse(id)
	assert.NoError(t, err)
	rows := ticksAtTip(t, repoDir, rel)
	require.Len(t, rows, 1)
	assert.Equal(t, id, rows[0].ID)
}

func Test_Tick_should_store_a_utc_timestamp(t *testing.T) {
	repoDir := setupFactory(t)
	rel := "repos/octo/app"
	startOfTest := time.Now().Add(-time.Second)
	id := runTick(t, repoDir)
	startOfCheck := time.Now().Add(time.Second)

	rows := ticksAtTip(t, repoDir, rel)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, id, row.ID)
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	require.NoError(t, err)
	assert.WithinRange(t, createdAt, startOfTest, startOfCheck)
}

func Test_Tick_should_commit_once_for_each_repo(t *testing.T) {
	app := appSetup{owner: "octo", name: "app"}
	other := appSetup{owner: "octo", name: "other"}
	apps := []appSetup{app, other}
	repoDir := setupRepos(t, apps)
	id := runTick(t, repoDir)

	output := mustGit(t, repoDir, "log -2 --format=%s state")
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.Len(t, lines, 2)
	appSubject := fmt.Sprintf("[%s] [tick] [octo/app] created", id)
	otherSubject := fmt.Sprintf("[%s] [tick] [octo/other] created", id)
	assert.Equal(t, otherSubject, lines[0])
	assert.Equal(t, appSubject, lines[1])
}

func Test_Tick_should_store_the_same_tick_in_each_repo(t *testing.T) {
	app := appSetup{owner: "octo", name: "app"}
	other := appSetup{owner: "octo", name: "other"}
	apps := []appSetup{app, other}
	repoDir := setupRepos(t, apps)
	appRel := "repos/octo/app"
	otherRel := "repos/octo/other"
	id := runTick(t, repoDir)

	appRows := ticksAtTip(t, repoDir, appRel)
	otherRows := ticksAtTip(t, repoDir, otherRel)
	require.Len(t, appRows, 1)
	require.Len(t, otherRows, 1)
	assert.Equal(t, id, appRows[0].ID)
	assert.Equal(t, id, otherRows[0].ID)
}

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	t.Chdir(repoDir)
	stdout := mustRoot(t, "tick")
	id = strings.TrimSpace(stdout)
	require.NotEmpty(t, id)
	return
}

type appSetup struct {
	owner string
	name  string
}

func setupFactory(t *testing.T) (repoDir string) {
	t.Helper()
	app := appSetup{owner: "octo", name: "app"}
	apps := []appSetup{app}
	repoDir = setupRepos(t, apps)
	return
}

func setupRepos(t *testing.T, apps []appSetup) (repoDir string) {
	t.Helper()
	repoDir = initFactory(t)
	for _, app := range apps {
		line := fmt.Sprintf("setup repo add --owner %s --name %s", app.owner, app.name)
		mustRoot(t, line)
	}
	commitConfig(t, repoDir)
	useStateBranch(t, repoDir)
	return
}

func initFactory(t *testing.T) (repoDir string) {
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
	t.Chdir(repoDir)
	return
}

func commitConfig(t *testing.T, repoDir string) {
	t.Helper()
	_ = mustGit(t, repoDir, "add -A")
	_ = mustGit(t, repoDir, "commit -m config")
}

func useStateBranch(t *testing.T, repoDir string) {
	t.Helper()
	_ = mustGit(t, repoDir, "checkout -b state")
	_ = mustGit(t, repoDir, "checkout main")
}

func mustRoot(t *testing.T, line string) (stdout string) {
	t.Helper()
	command := root.NewRoot()
	var out bytes.Buffer
	var errOut bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&errOut)
	wrapped := commands.NewCmd(command)
	err := wrapped.Shell(t.Context(), line)
	require.NoError(t, err)
	stdout = out.String()
	return
}

func ticksAtTip(t *testing.T, repoDir string, rel string) (rows []records.Tick) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	spec := "state:" + filepath.ToSlash(rel) + "/state.db"
	showLine := fmt.Sprintf("show %s", spec)
	blob := mustGit(t, repoDir, showLine)
	err := os.WriteFile(tipPath, blob, 0644)
	require.NoError(t, err)

	db, err := statedb.Open(t.Context(), tipPath)
	require.NoError(t, err)
	defer db.Close()

	rows, err = db.Queries.GetTicks(t.Context())
	require.NoError(t, err)
	return
}
