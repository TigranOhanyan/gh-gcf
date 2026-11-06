package executor_test

import (
	"bytes"
	"context"
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
	"github.com/TigranOhanyan/gh-gcf/commands/executor"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

func Test_Execute_should_commit_the_tick_subject_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	step := recordStep{}

	exec := executor.StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), step)
	require.NoError(t, err)

	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("✅ [%s] [record] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	expectedBody := fmt.Sprintf("✅ recorded tick %s", id)
	assert.Equal(t, expectedBody, strings.TrimSpace(string(body)))
}

func Test_Execute_should_store_one_step_named_for_the_step(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	step := recordStep{}

	exec := executor.StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), step)
	require.NoError(t, err)

	rows := stepsAtTip(t, repoDir, "repos/octo/app")
	require.Len(t, rows, 1)
	row := rows[0]
	_, err = ulid.Parse(row.ID)
	assert.NoError(t, err)
	assert.Equal(t, id, row.TickID)
	assert.Equal(t, recordName, row.Name)
}

func Test_Execute_should_store_a_utc_timestamp_for_the_step(t *testing.T) {
	startOfTest := time.Now().Add(-time.Second)
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	step := recordStep{}

	exec := executor.StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), step)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	rows := stepsAtTip(t, repoDir, "repos/octo/app")
	require.Len(t, rows, 1)
	row := rows[0]
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	require.NoError(t, err)
	assert.WithinRange(t, createdAt, startOfTest, startOfCheck)
}

func Test_Execute_should_commit_once_for_each_repo(t *testing.T) {
	app := appSetup{owner: "octo", name: "app"}
	other := appSetup{owner: "octo", name: "other"}
	apps := []appSetup{app, other}
	repoDir := setupRepos(t, apps)
	id := runTick(t, repoDir)
	step := recordStep{}

	exec := executor.StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), step)
	require.NoError(t, err)

	output := mustGit(t, repoDir, "log -2 --format=%s state")
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.Len(t, lines, 2)
	appSubject := fmt.Sprintf("✅ [%s] [record] [octo/app]", id)
	otherSubject := fmt.Sprintf("✅ [%s] [record] [octo/other]", id)
	assert.Equal(t, otherSubject, lines[0])
	assert.Equal(t, appSubject, lines[1])
}

func Test_Execute_should_refuse_a_path_that_is_not_repos_owner_name(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	stray := filepath.Join(repoDir, "repos", "stray.txt")
	err := os.WriteFile(stray, []byte("x"), 0644)
	require.NoError(t, err)
	step := recordStep{}

	exec := executor.StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err = exec.Execute(t.Context(), step)
	require.Error(t, err)

	message := mustGit(t, repoDir, "log -1 --format=%s state")
	expected := fmt.Sprintf("[%s] [tick] [octo/app] created", id)
	assert.Equal(t, expected, strings.TrimSpace(string(message)))
}

func Test_Execute_should_commit_when_the_step_writes_nothing(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	step := quietStep{}

	exec := executor.StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), step)
	require.NoError(t, err)

	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("✅ [%s] [quiet] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	expectedBody := "✅ wrote nothing"
	assert.Equal(t, expectedBody, strings.TrimSpace(string(body)))
}

func Test_Execute_should_commit_the_error_when_the_step_returns_an_error(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	step := failStep{}

	exec := executor.StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), step)
	require.Error(t, err)

	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("❌ [%s] [fail] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	expectedBody := "❌ label is missing"
	assert.Equal(t, expectedBody, strings.TrimSpace(string(body)))
}

func Test_Execute_should_keep_the_message_and_the_error(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	step := bothStep{}

	exec := executor.StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), step)
	require.Error(t, err)

	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("❌ [%s] [both] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	expectedBody := "✅ [todo] created\n❌ [bug] POST failed"
	assert.Equal(t, expectedBody, strings.TrimSpace(string(body)))
}

func Test_Execute_should_store_the_step_when_the_step_returns_an_error(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	step := failStep{}

	exec := executor.StepExecutor{
		FactoryDir:   repoDir,
		ConfigBranch: "main",
		StateBranch:  "state",
		TickID:       id,
	}
	err := exec.Execute(t.Context(), step)
	require.Error(t, err)

	rows := stepsAtTip(t, repoDir, "repos/octo/app")
	require.Len(t, rows, 1)
	assert.Equal(t, id, rows[0].TickID)
	assert.Equal(t, failName, rows[0].Name)
}

func Test_Execute_should_refuse_a_missing_tick_id(t *testing.T) {
	step := recordStep{}

	exec := executor.StepExecutor{
		FactoryDir:   filepath.Join(JunkDir, "missing-tick"),
		ConfigBranch: "main",
		StateBranch:  "state",
	}
	err := exec.Execute(t.Context(), step)
	require.Error(t, err)
	assert.EqualError(t, err, "tick id is required")
}

const recordName = "record"

type recordStep struct{}

func (recordStep) Name() (name string) {
	name = recordName
	return
}

func (recordStep) Apply(_ context.Context, _ commands.Repo, tickRow records.Tick, _ records.Step) (message string, err error) {
	id := tickRow.ID
	message = fmt.Sprintf("recorded tick %s", id)
	return
}

const quietName = "quiet"
const quietMessage = "wrote nothing"

type quietStep struct{}

func (quietStep) Name() (name string) {
	name = quietName
	return
}

func (quietStep) Apply(context.Context, commands.Repo, records.Tick, records.Step) (message string, err error) {
	message = quietMessage
	return
}

const failName = "fail"

type failStep struct{}

func (failStep) Name() (name string) {
	name = failName
	return
}

func (failStep) Apply(context.Context, commands.Repo, records.Tick, records.Step) (message string, err error) {
	err = fmt.Errorf("label is missing")
	return
}

const bothName = "both"

type bothStep struct{}

func (bothStep) Name() (name string) {
	name = bothName
	return
}

func (bothStep) Apply(context.Context, commands.Repo, records.Tick, records.Step) (message string, err error) {
	message = "[todo] created"
	err = fmt.Errorf("[bug] POST failed")
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

func stepsAtTip(t *testing.T, repoDir string, rel string) (rows []records.Step) {
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

	rows, err = db.Queries.GetSteps(t.Context())
	require.NoError(t, err)
	return
}

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	t.Chdir(repoDir)
	stdout := mustRoot(t, "tick")
	id = strings.TrimSpace(stdout)
	require.NotEmpty(t, id)
	return
}
