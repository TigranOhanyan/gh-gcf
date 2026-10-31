package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/tick"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	stateentities "github.com/TigranOhanyan/gh-gcf/statedb/entities"
)

func Test_Execute_should_commit_the_tick_subject_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	action := recordAction{}

	exec := StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), action)
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expected := fmt.Sprintf("[%s] [record] [octo/app] recorded tick %s", id, id)
	assert.Equal(t, expected, strings.TrimSpace(string(message)))
}

func Test_Execute_should_commit_once_for_each_repo(t *testing.T) {
	app := appSetup{owner: "octo", name: "app"}
	other := appSetup{owner: "octo", name: "other"}
	apps := []appSetup{app, other}
	repoDir := setupRepos(t, apps)
	id := runTick(t, repoDir)
	action := recordAction{}

	exec := StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), action)
	require.NoError(t, err)

	output := mustGit(t, repoDir, "log", "-2", "--format=%s", "state")
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.Len(t, lines, 2)
	appSubject := fmt.Sprintf("[%s] [record] [octo/app] recorded tick %s", id, id)
	otherSubject := fmt.Sprintf("[%s] [record] [octo/other] recorded tick %s", id, id)
	assert.Equal(t, otherSubject, lines[0])
	assert.Equal(t, appSubject, lines[1])
}

func Test_Execute_should_refuse_a_path_that_is_not_repos_owner_name(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	stray := filepath.Join(repoDir, "repos", "stray.txt")
	err := os.WriteFile(stray, []byte("x"), 0644)
	require.NoError(t, err)
	action := recordAction{}

	exec := StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err = exec.Execute(t.Context(), action)
	require.Error(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expected := fmt.Sprintf("[%s] [tick] [octo/app] created", id)
	assert.Equal(t, expected, strings.TrimSpace(string(message)))
}

func Test_Execute_should_commit_when_the_action_writes_nothing(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	action := quietAction{}

	exec := StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), action)
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expected := fmt.Sprintf("[%s] [quiet] [octo/app] wrote nothing", id)
	assert.Equal(t, expected, strings.TrimSpace(string(message)))
}

func Test_Execute_should_commit_the_error_when_the_action_returns_an_error(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	action := failAction{}

	exec := StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), action)
	require.Error(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expected := fmt.Sprintf("[%s] [fail] [octo/app] label is missing", id)
	assert.Equal(t, expected, strings.TrimSpace(string(message)))
}

func Test_Execute_should_refuse_a_missing_tick_id(t *testing.T) {
	action := recordAction{}

	exec := StepExecutor{
		FactoryDir:   filepath.Join(JunkDir, "missing-tick"),
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), action)
	require.Error(t, err)
	assert.EqualError(t, err, "tick id is required")
}

const recordName = "record"

type recordAction struct{}

func (recordAction) Name() (name string) {
	name = recordName
	return
}

func (recordAction) Apply(_ context.Context, _ commands.Repo, tickRow stateentities.Tick) (message string, err error) {
	id := tickRow.ID
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

func (quietAction) Apply(context.Context, commands.Repo, stateentities.Tick) (message string, err error) {
	message = quietMessage
	return
}

const failName = "fail"

type failAction struct{}

func (failAction) Name() (name string) {
	name = failName
	return
}

func (failAction) Apply(context.Context, commands.Repo, stateentities.Tick) (message string, err error) {
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

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	cmd := tick.NewTick()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	args := []string{"--factory-repo", repoDir}
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	id = strings.TrimSpace(output.String())
	require.NotEmpty(t, id)
	return
}
