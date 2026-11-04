package tick

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

	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

func Test_Tick_should_print_the_stored_tick_id(t *testing.T) {
	repoDir := setupFactory(t)
	t.Chdir(repoDir)
	cmd := NewTick()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	id := strings.TrimSpace(output.String())
	_, err = ulid.Parse(id)
	assert.NoError(t, err)
	rows := ticksAtTip(t, repoDir, "repos/octo/app")
	require.Len(t, rows, 1)
	assert.Equal(t, id, rows[0].ID)
}

func Test_CreateTick_should_commit_the_tick_subject_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)
	tick := tickExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}

	id, err := tick.CreateTick(t.Context())
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expected := fmt.Sprintf("[%s] [tick] [octo/app] created", id)
	assert.Equal(t, expected, strings.TrimSpace(string(message)))
}

func Test_CreateTick_should_store_one_tick_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)
	rel := "repos/octo/app"
	tick := tickExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}

	id, err := tick.CreateTick(t.Context())
	require.NoError(t, err)

	_, err = ulid.Parse(id)
	assert.NoError(t, err)
	rows := ticksAtTip(t, repoDir, rel)
	require.Len(t, rows, 1)
	assert.Equal(t, id, rows[0].ID)
}

func Test_CreateTick_should_store_a_utc_timestamp(t *testing.T) {
	repoDir := setupFactory(t)
	rel := "repos/octo/app"
	tick := tickExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}

	startOfTest := time.Now().Add(-time.Second)
	id, err := tick.CreateTick(t.Context())
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	rows := ticksAtTip(t, repoDir, rel)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, id, row.ID)
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	require.NoError(t, err)
	assert.WithinRange(t, createdAt, startOfTest, startOfCheck)
}

func Test_CreateTick_should_commit_once_for_each_repo(t *testing.T) {
	app := appSetup{owner: "octo", name: "app"}
	other := appSetup{owner: "octo", name: "other"}
	apps := []appSetup{app, other}
	repoDir := setupRepos(t, apps)
	tick := tickExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}

	id, err := tick.CreateTick(t.Context())
	require.NoError(t, err)

	output := mustGit(t, repoDir, "log", "-2", "--format=%s", "state")
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.Len(t, lines, 2)
	appSubject := fmt.Sprintf("[%s] [tick] [octo/app] created", id)
	otherSubject := fmt.Sprintf("[%s] [tick] [octo/other] created", id)
	assert.Equal(t, otherSubject, lines[0])
	assert.Equal(t, appSubject, lines[1])
}

func Test_CreateTick_should_store_the_same_tick_in_each_repo(t *testing.T) {
	app := appSetup{owner: "octo", name: "app"}
	other := appSetup{owner: "octo", name: "other"}
	apps := []appSetup{app, other}
	repoDir := setupRepos(t, apps)
	appRel := "repos/octo/app"
	otherRel := "repos/octo/other"
	tick := tickExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}

	id, err := tick.CreateTick(t.Context())
	require.NoError(t, err)

	appRows := ticksAtTip(t, repoDir, appRel)
	otherRows := ticksAtTip(t, repoDir, otherRel)
	require.Len(t, appRows, 1)
	require.Len(t, otherRows, 1)
	assert.Equal(t, id, appRows[0].ID)
	assert.Equal(t, id, otherRows[0].ID)
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
	ctx := t.Context()
	repoDir = filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	_ = mustGit(t, repoDir, "init", "-b", "main")
	_ = mustGit(t, repoDir, "config", "user.email", "test@example.com")
	_ = mustGit(t, repoDir, "config", "user.name", "test")

	for _, app := range apps {
		configPath := filepath.Join(repoDir, "repos", app.owner, app.name, "config.db")
		configDir := filepath.Dir(configPath)
		err = os.MkdirAll(configDir, 0755)
		require.NoError(t, err)
		configDB, openErr := configdb.OpenAndMigrate(ctx, configPath)
		require.NoError(t, openErr)
		err = configDB.Close()
		require.NoError(t, err)
		configRel := filepath.Join("repos", app.owner, app.name, "config.db")
		_ = mustGit(t, repoDir, "add", configRel)
	}
	_ = mustGit(t, repoDir, "commit", "-m", "config")

	_ = mustGit(t, repoDir, "checkout", "-b", "state")
	for _, app := range apps {
		statePath := filepath.Join(repoDir, "repos", app.owner, app.name, "state.db")
		stateDB, openErr := statedb.Open(ctx, statePath)
		require.NoError(t, openErr)
		err = stateDB.Close()
		require.NoError(t, err)
		stateRel := filepath.Join("repos", app.owner, app.name, "state.db")
		_ = mustGit(t, repoDir, "add", stateRel)
	}
	_ = mustGit(t, repoDir, "commit", "-m", "state")

	_ = mustGit(t, repoDir, "checkout", "main")
	return
}

func ticksAtTip(t *testing.T, repoDir string, rel string) (rows []records.Tick) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	spec := "state:" + filepath.ToSlash(rel) + "/state.db"
	blob := mustGit(t, repoDir, "show", spec)
	err := os.WriteFile(tipPath, blob, 0644)
	require.NoError(t, err)

	db, err := statedb.Open(t.Context(), tipPath)
	require.NoError(t, err)
	defer db.Close()

	rows, err = db.Queries.GetTicks(t.Context())
	require.NoError(t, err)
	return
}
