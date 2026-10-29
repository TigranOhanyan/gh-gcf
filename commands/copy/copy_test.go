package copy

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	configentities "github.com/TigranOhanyan/gh-gcf/configdb/entities"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	stateentities "github.com/TigranOhanyan/gh-gcf/statedb/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Copy_should_commit_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)

	runCopy(t, repoDir)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	assert.Equal(t, "copy config into state", strings.TrimSpace(string(message)))
}

func Test_Copy_should_apply_config_rows_on_the_state_branch(t *testing.T) {
	repoDir := setupFactory(t)

	runCopy(t, repoDir)

	rows := readCommittedStates(t, repoDir)
	expected := []stateentities.State{
		{Name: "starting-label", Value: "todo"},
		{Name: "attention-label", Value: "needs-attention"},
	}
	assert.ElementsMatch(t, expected, rows)
}

func runCopy(t *testing.T, repoDir string) {
	t.Helper()
	cmd := NewCopy()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	args := []string{
		"--factory-repo", repoDir,
		"--config-branch", "main",
		"--state-branch", "state",
	}
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
}

func setupFactory(t *testing.T) (repoDir string) {
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

	configPath := filepath.Join(repoDir, "config.db")
	configDB, err := configdb.OpenAndMigrate(ctx, configPath)
	require.NoError(t, err)

	starting := configentities.CreateConfigParams{
		Name:  "starting-label",
		Value: "todo",
	}
	err = configDB.Queries.CreateConfig(ctx, starting)
	require.NoError(t, err)

	attention := configentities.CreateConfigParams{
		Name:  "attention-label",
		Value: "needs-attention",
	}
	err = configDB.Queries.CreateConfig(ctx, attention)
	require.NoError(t, err)

	err = configDB.Close()
	require.NoError(t, err)
	_ = mustGit(t, repoDir, "add", "config.db")
	_ = mustGit(t, repoDir, "commit", "-m", "config")

	_ = mustGit(t, repoDir, "checkout", "-b", "state")
	statePath := filepath.Join(repoDir, "state.db")
	stateDB, err := statedb.Open(ctx, statePath)
	require.NoError(t, err)
	err = stateDB.Close()
	require.NoError(t, err)
	_ = mustGit(t, repoDir, "add", "state.db")
	_ = mustGit(t, repoDir, "commit", "-m", "state")

	_ = mustGit(t, repoDir, "checkout", "main")
	return
}

func readCommittedStates(t *testing.T, repoDir string) (rows []stateentities.State) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	blob := mustGit(t, repoDir, "show", "state:state.db")
	err := os.WriteFile(tipPath, blob, 0644)
	require.NoError(t, err)
	rows = readStates(t, tipPath)
	return
}

func readStates(t *testing.T, path string) (rows []stateentities.State) {
	t.Helper()
	ctx := t.Context()
	db, err := statedb.Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	rows, err = db.Queries.GetStates(ctx)
	require.NoError(t, err)
	return
}
