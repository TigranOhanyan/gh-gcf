package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	configentities "github.com/TigranOhanyan/gh-gcf/configdb/entities"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	stateentities "github.com/TigranOhanyan/gh-gcf/statedb/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Copy_should_copy_each_config_row_into_state(t *testing.T) {
	ctx := t.Context()
	configPath, statePath := setupDatabases(t)

	err := Copy(ctx, configPath, statePath)
	require.NoError(t, err)

	rows := readStates(t, statePath)
	expected := []stateentities.State{
		{Name: "starting-label", Value: "todo"},
		{Name: "attention-label", Value: "needs-attention"},
	}
	assert.ElementsMatch(t, expected, rows)
}

func Test_CopyCommand_should_copy_each_config_row_into_state(t *testing.T) {
	ctx := t.Context()
	configPath, statePath := setupDatabases(t)

	cmd := NewRoot()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	args := []string{"copy", "--config", configPath, "--state", statePath}
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(ctx)
	require.NoError(t, err)

	rows := readStates(t, statePath)
	expected := []stateentities.State{
		{Name: "starting-label", Value: "todo"},
		{Name: "attention-label", Value: "needs-attention"},
	}
	assert.ElementsMatch(t, expected, rows)
}

func setupDatabases(t *testing.T) (configPath string, statePath string) {
	t.Helper()
	ctx := t.Context()
	configPath = filepath.Join(JunkDir, t.Name()+"-config.db")
	statePath = filepath.Join(JunkDir, t.Name()+"-state.db")
	removeDatabase(t, configPath)
	removeDatabase(t, statePath)

	configDB, err := configdb.Open(ctx, configPath)
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

	stateDB, err := statedb.Open(ctx, statePath)
	require.NoError(t, err)
	err = stateDB.Close()
	require.NoError(t, err)
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

func removeDatabase(t *testing.T, path string) {
	t.Helper()
	err := os.Remove(path)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		require.NoError(t, err)
	}
}
