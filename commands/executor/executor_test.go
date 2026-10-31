package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	stateentities "github.com/TigranOhanyan/gh-gcf/statedb/entities"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Execute_should_commit_the_tick_subject_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)
	action := recordAction{}

	exec := Executor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), action)
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	ids := ticksAtTip(t, repoDir, "repos/octo/app")
	require.Len(t, ids, 1)
	id := ids[0]
	expected := fmt.Sprintf("[%s] [record] [octo/app] recorded tick %s", id, id)
	assert.Equal(t, expected, strings.TrimSpace(string(message)))
}

func Test_Execute_should_store_one_tick_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)
	action := recordAction{}
	rel := "repos/octo/app"

	exec := Executor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), action)
	require.NoError(t, err)

	ids := ticksAtTip(t, repoDir, rel)
	require.Len(t, ids, 1)
	_, err = ulid.Parse(ids[0])
	assert.NoError(t, err)
}

func Test_Execute_should_commit_once_for_each_repo(t *testing.T) {
	app := appSetup{owner: "octo", name: "app"}
	other := appSetup{owner: "octo", name: "other"}
	apps := []appSetup{app, other}
	repoDir := setupRepos(t, apps)
	action := recordAction{}

	exec := Executor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), action)
	require.NoError(t, err)

	output := mustGit(t, repoDir, "log", "-2", "--format=%s", "state")
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.Len(t, lines, 2)
	appIDs := ticksAtTip(t, repoDir, "repos/octo/app")
	otherIDs := ticksAtTip(t, repoDir, "repos/octo/other")
	require.Len(t, appIDs, 1)
	require.Len(t, otherIDs, 1)
	appID := appIDs[0]
	otherID := otherIDs[0]
	appSubject := fmt.Sprintf("[%s] [record] [octo/app] recorded tick %s", appID, appID)
	otherSubject := fmt.Sprintf("[%s] [record] [octo/other] recorded tick %s", otherID, otherID)
	assert.Equal(t, otherSubject, lines[0])
	assert.Equal(t, appSubject, lines[1])
}

func Test_Execute_should_store_one_tick_for_each_repo(t *testing.T) {
	app := appSetup{owner: "octo", name: "app"}
	other := appSetup{owner: "octo", name: "other"}
	apps := []appSetup{app, other}
	repoDir := setupRepos(t, apps)
	action := recordAction{}
	appRel := "repos/octo/app"
	otherRel := "repos/octo/other"

	exec := Executor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), action)
	require.NoError(t, err)

	appIDs := ticksAtTip(t, repoDir, appRel)
	otherIDs := ticksAtTip(t, repoDir, otherRel)
	require.Len(t, appIDs, 1)
	require.Len(t, otherIDs, 1)
	_, err = ulid.Parse(appIDs[0])
	assert.NoError(t, err)
	_, err = ulid.Parse(otherIDs[0])
	assert.NoError(t, err)
	assert.NotEqual(t, appIDs[0], otherIDs[0])
}

func Test_Execute_should_refuse_a_path_that_is_not_repos_owner_name(t *testing.T) {
	repoDir := setupFactory(t)
	stray := filepath.Join(repoDir, "repos", "stray.txt")
	err := os.WriteFile(stray, []byte("x"), 0644)
	require.NoError(t, err)
	action := recordAction{}

	exec := Executor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err = exec.Execute(t.Context(), action)
	require.Error(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	assert.Equal(t, "state", strings.TrimSpace(string(message)))
}

func Test_Execute_should_commit_when_the_action_writes_nothing(t *testing.T) {
	repoDir := setupFactory(t)
	action := quietAction{}

	exec := Executor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), action)
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	ids := ticksAtTip(t, repoDir, "repos/octo/app")
	require.Len(t, ids, 1)
	id := ids[0]
	expected := fmt.Sprintf("[%s] [quiet] [octo/app] wrote nothing", id)
	assert.Equal(t, expected, strings.TrimSpace(string(message)))
}

func Test_Execute_should_store_one_tick_when_the_action_writes_nothing(t *testing.T) {
	repoDir := setupFactory(t)
	action := quietAction{}
	rel := "repos/octo/app"

	exec := Executor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), action)
	require.NoError(t, err)

	ids := ticksAtTip(t, repoDir, rel)
	require.Len(t, ids, 1)
	_, err = ulid.Parse(ids[0])
	assert.NoError(t, err)
}

func Test_Execute_should_leave_the_branch_when_the_action_returns_an_error(t *testing.T) {
	repoDir := setupFactory(t)
	action := failAction{}

	exec := Executor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), action)
	require.Error(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	assert.Equal(t, "state", strings.TrimSpace(string(message)))
}

func Test_Execute_should_not_store_a_tick_when_the_action_returns_an_error(t *testing.T) {
	repoDir := setupFactory(t)
	action := failAction{}
	rel := "repos/octo/app"

	exec := Executor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), action)
	require.Error(t, err)

	ids := ticksAtTip(t, repoDir, rel)
	assert.Empty(t, ids)
}

const recordName = "record"

type recordAction struct{}

func (recordAction) Name() (name string) {
	name = recordName
	return
}

func (recordAction) Apply(_ context.Context, _ Repo, tick stateentities.Tick) (message string, err error) {
	id := tick.ID
	message = fmt.Sprintf("recorded tick %s", id)
	return
}

const quietName = "quiet"
const quietMessage = "wrote nothing"

type quietAction struct{}

func (quietAction) Name() (name string) {
	name = quietName
	return
}

func (quietAction) Apply(context.Context, Repo, stateentities.Tick) (message string, err error) {
	message = quietMessage
	return
}

const failName = "fail"

type failAction struct{}

func (failAction) Name() (name string) {
	name = failName
	return
}

func (failAction) Apply(context.Context, Repo, stateentities.Tick) (message string, err error) {
	err = fmt.Errorf("label is missing")
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

func ticksAtTip(t *testing.T, repoDir string, rel string) (ids []string) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	spec := "state:" + filepath.ToSlash(rel) + "/state.db"
	blob := mustGit(t, repoDir, "show", spec)
	err := os.WriteFile(tipPath, blob, 0644)
	require.NoError(t, err)

	db, err := statedb.Open(t.Context(), tipPath)
	require.NoError(t, err)
	defer db.Close()

	ids, err = db.Queries.GetTicks(t.Context())
	require.NoError(t, err)
	return
}
